package metricsmw_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/presentation/http/metricsmw"
	"github.com/scality/static-oci-registry/pkg/presentation/http/reqlabels"
)

// buildMetrics constructs a fresh pair of request vecs registered on a
// process-local registry so each test observation is isolated from the
// package-level RegistryRequestMetrics singleton.
func buildMetrics(t *testing.T) (*prometheus.Registry, *domain.RequestMetrics) {
	t.Helper()

	reg := prometheus.NewRegistry()
	m := &domain.RequestMetrics{
		Requests: domain.NewRequestsCounter(),
		Duration: domain.NewRequestDuration(),
	}
	reg.MustRegister(m.Requests, m.Duration)

	return reg, m
}

// findSample scans a MetricFamily for a single sample matching the given
// label set (exact match on the labels listed; unlisted labels are ignored).
// Returns the sample's counter value or histogram sample count.
func findSample(t *testing.T, mf *dto.MetricFamily, wantLabels map[string]string) *dto.Metric {
	t.Helper()

	for _, m := range mf.GetMetric() {
		if labelsMatch(m.GetLabel(), wantLabels) {
			return m
		}
	}

	return nil
}

func labelsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) < len(want) {
		return false
	}

	pairs := make(map[string]string, len(got))
	for _, lp := range got {
		pairs[lp.GetName()] = lp.GetValue()
	}

	for k, v := range want {
		if pairs[k] != v {
			return false
		}
	}

	return true
}

func gather(t *testing.T, reg *prometheus.Registry, name string) *dto.MetricFamily {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}

	t.Fatalf("metric family %q not found; families=%v", name, families)

	return nil
}

func TestWrap_capturesAllLabels(t *testing.T) {
	reg, m := buildMetrics(t)

	// Handler that mimics an OCI handler: reads the labels bag, writes both
	// solution labels, then writes a 200 body.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labels := reqlabels.From(r.Context())
		if labels == nil {
			t.Error("inner handler: reqlabels.From returned nil; middleware did not install the bag")
			return
		}

		labels.SetEndpoint("pull_blob")
		labels.SetSolution(domain.SolutionVersion{Solution: "acme", Version: "2.3.4"})
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	handler := metricsmw.Wrap(inner, m, "reg-1")

	req := httptest.NewRequest(http.MethodGet, "/v2/hello/blobs/sha256:abcd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	want := map[string]string{
		domain.LabelMethod:          "get",
		domain.LabelCode:            "200",
		domain.LabelEndpoint:        "pull_blob",
		domain.LabelSolutionName:    "acme",
		domain.LabelSolutionVersion: "2.3.4",
		domain.LabelRegistry:        "reg-1",
	}

	counters := gather(t, reg, "registry_http_requests_total")
	if sample := findSample(t, counters, want); sample == nil {
		t.Fatalf("no counter sample matched %v; got %v", want, counters.GetMetric())
	} else if v := sample.GetCounter().GetValue(); v != 1 {
		t.Fatalf("counter value = %v, want 1", v)
	}

	histogram := gather(t, reg, "registry_http_request_duration_seconds")
	if sample := findSample(t, histogram, want); sample == nil {
		t.Fatalf("no histogram sample matched %v; got %v", want, histogram.GetMetric())
	} else if v := sample.GetHistogram().GetSampleCount(); v != 1 {
		t.Fatalf("histogram sample count = %v, want 1", v)
	}
}

func TestWrap_missingSolutionYieldsEmptyLabels(t *testing.T) {
	// Simulates ListTags-shaped handlers that aggregate across solutions:
	// the bag stays empty, so solution_name/solution_version must be "".
	reg, m := buildMetrics(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqlabels.From(r.Context()).SetEndpoint("list_tags")
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsmw.Wrap(inner, m, "reg-x")

	req := httptest.NewRequest(http.MethodGet, "/v2/img/tags/list", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	want := map[string]string{
		domain.LabelMethod:          "get",
		domain.LabelCode:            "200",
		domain.LabelEndpoint:        "list_tags",
		domain.LabelSolutionName:    "",
		domain.LabelSolutionVersion: "",
		domain.LabelRegistry:        "reg-x",
	}

	counters := gather(t, reg, "registry_http_requests_total")
	if sample := findSample(t, counters, want); sample == nil {
		t.Fatalf("no counter sample matched %v; got %v", want, counters.GetMetric())
	}
}

func TestWrap_capturesErrorCode(t *testing.T) {
	reg, m := buildMetrics(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqlabels.From(r.Context()).SetEndpoint("fetch_manifest")
		w.WriteHeader(http.StatusNotFound)
	})

	handler := metricsmw.Wrap(inner, m, "r")

	req := httptest.NewRequest(http.MethodHead, "/v2/img/manifests/absent", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	want := map[string]string{
		domain.LabelMethod:   "head",
		domain.LabelCode:     "404",
		domain.LabelEndpoint: "fetch_manifest",
		domain.LabelRegistry: "r",
	}

	counters := gather(t, reg, "registry_http_requests_total")
	if sample := findSample(t, counters, want); sample == nil {
		t.Fatalf("no counter sample matched %v; got %v", want, counters.GetMetric())
	}
}

func TestWrap_endpointFallbackWhenNoRouteMatched(t *testing.T) {
	// Simulates the router's fall-through branch: nothing calls SetEndpoint
	// on the bag, so the middleware must surface the sentinel value instead
	// of "" to bound cardinality on unmatched /v2/ paths.
	reg, m := buildMetrics(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := metricsmw.Wrap(inner, m, "reg-z")

	req := httptest.NewRequest(http.MethodGet, "/v2/no-such-path", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	want := map[string]string{
		domain.LabelMethod:   "get",
		domain.LabelCode:     "404",
		domain.LabelEndpoint: domain.EndpointUnknown,
		domain.LabelRegistry: "reg-z",
	}

	counters := gather(t, reg, "registry_http_requests_total")
	if sample := findSample(t, counters, want); sample == nil {
		t.Fatalf("no counter sample matched %v; got %v", want, counters.GetMetric())
	}

	for _, sample := range counters.GetMetric() {
		for _, lp := range sample.GetLabel() {
			if lp.GetName() == domain.LabelEndpoint && lp.GetValue() == "" {
				t.Fatalf("found a counter series with endpoint=\"\": %v", sample.GetLabel())
			}
		}
	}
}
