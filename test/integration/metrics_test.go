// metrics_test.go exercises the integration wiring between the OCI router and
// the dedicated Prometheus scrape listener.
package integration

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/test/utils"
)

const (
	requestsTotalMetric   = "registry_http_requests_total"
	requestDurationMetric = "registry_http_request_duration_seconds"
)

var _ = Describe("Metrics endpoint", Ordered, func() {
	var (
		metricsClient *http.Client
		ociClient     *http.Client
	)

	BeforeEach(func() {
		metricsClient = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
		}
		ociClient = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	Context("/metrics endpoint plumbing", func() {
		It("serves Prometheus text format over plain HTTP", func() {
			resp, body := scrapeMetricsBody(metricsClient)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix("text/plain"))

			metrics := string(body)
			Expect(metrics).To(ContainSubstring("# HELP registry_http_requests_total"))
			Expect(metrics).To(ContainSubstring("# TYPE registry_http_requests_total counter"))
			Expect(metrics).To(ContainSubstring("# HELP registry_http_request_duration_seconds"))
			Expect(metrics).To(ContainSubstring("# TYPE registry_http_request_duration_seconds histogram"))
		})
	})

	Context("RED metric values on /v2/ traffic", Ordered, func() {
		const requestCount = 3

		re := &utils.RegistryEntry{
			Solution: "metrics-red-solution",
			Version:  "v1.0.0",
			Image:    "docker.io/library/alpine",
			Tag:      "3.22.2",
		}
		wantLabels := map[string]string{
			domain.LabelMethod:          strings.ToLower(http.MethodGet),
			domain.LabelCode:            "200",
			domain.LabelEndpoint:        "list_tags",
			domain.LabelSolutionName:    "",
			domain.LabelSolutionVersion: "",
			domain.LabelRegistry:        registryName,
		}

		BeforeAll(func() {
			suite.BuildImage(re)
		})

		AfterAll(func() {
			suite.ClearImage(re)
		})

		It("increments counter and histogram count with the configured registry label", func() {
			before := scrapeMetrics(metricsClient)
			beforeCounter, _ := utils.FindCounter(before[requestsTotalMetric], wantLabels)
			beforeHistogramCount, _ := utils.FindHistogramCount(before[requestDurationMetric], wantLabels)

			for range requestCount {
				req := initRequest(string(re.Image), "/tags/list", nil)

				resp, _ := execRequest(ociClient, req)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			}

			after := scrapeMetrics(metricsClient)

			afterCounter, ok := utils.FindCounter(after[requestsTotalMetric], wantLabels)
			Expect(ok).To(BeTrue())
			Expect(afterCounter - beforeCounter).To(Equal(float64(requestCount)))

			afterHistogramCount, ok := utils.FindHistogramCount(after[requestDurationMetric], wantLabels)
			Expect(ok).To(BeTrue())
			Expect(afterHistogramCount - beforeHistogramCount).To(Equal(uint64(requestCount)))
		})
	})

	Context("Error labelling", func() {
		It("labels route errors by status code, endpoint, and method", func() {
			req := initRequest("nonexistent/image", "/tags/list", nil)

			resp, body := execRequest(ociClient, req)

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			checkErrorResponse(body, ocierrors.NameUnknown)

			req = initRequestWithMethod(http.MethodPut, "docker.io/library/alpine", "/tags/list", nil)

			resp, body = execRequest(ociClient, req)

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			checkErrorResponse(body, ocierrors.Unsupported)

			families := scrapeMetrics(metricsClient)
			getLabels := map[string]string{
				domain.LabelMethod:   strings.ToLower(http.MethodGet),
				domain.LabelCode:     "404",
				domain.LabelEndpoint: "list_tags",
				domain.LabelRegistry: registryName,
			}
			putLabels := map[string]string{
				domain.LabelMethod:   strings.ToLower(http.MethodPut),
				domain.LabelCode:     "404",
				domain.LabelEndpoint: "list_tags",
				domain.LabelRegistry: registryName,
			}

			getCounter, ok := utils.FindCounter(families[requestsTotalMetric], getLabels)
			Expect(ok).To(BeTrue())
			Expect(getCounter).To(BeNumerically(">", 0))

			putCounter, ok := utils.FindCounter(families[requestsTotalMetric], putLabels)
			Expect(ok).To(BeTrue())
			Expect(putCounter).To(BeNumerically(">", 0))
		})
	})

	Context("Route exclusion", func() {
		It("does not add OCI request metrics for health probes or metrics scrapes", func() {
			before := scrapeMetrics(metricsClient)
			excludedEndpoints := map[string]struct{}{
				"healthz": {},
				"readyz":  {},
				"metrics": {},
			}
			beforeCounters := counterSumForEndpoints(before[requestsTotalMetric], excludedEndpoints)
			beforeHistograms := histogramCountSumForEndpoints(before[requestDurationMetric], excludedEndpoints)

			expectOK(ociClient, "https://localhost"+cfg.HTTP.Addr+"/healthz")
			expectOK(ociClient, "https://localhost"+cfg.HTTP.Addr+"/readyz")
			expectOK(metricsClient, metricsAddr+"/metrics")

			after := scrapeMetrics(metricsClient)

			Expect(counterSumForEndpoints(after[requestsTotalMetric], excludedEndpoints)).To(Equal(beforeCounters))
			Expect(histogramCountSumForEndpoints(after[requestDurationMetric], excludedEndpoints)).To(Equal(beforeHistograms))
		})
	})

	Context("Unknown route fallback", func() {
		It("emits a counter with endpoint=\"unknown\" when the /v2/ path matches no route", func() {
			wantLabels := map[string]string{
				domain.LabelMethod:   strings.ToLower(http.MethodGet),
				domain.LabelCode:     "404",
				domain.LabelEndpoint: domain.EndpointUnknown,
				domain.LabelRegistry: registryName,
			}

			before := scrapeMetrics(metricsClient)
			beforeCounter, _ := utils.FindCounter(before[requestsTotalMetric], wantLabels)

			// "no-such-route" is a single path segment under /v2/, which
			// fails every registered Route.Matches (list_tags requires
			// /tags/list, fetch_manifest requires /manifests/, pull_blob
			// requires /blobs/). Router falls through to http.NotFound.
			req := initRequest("no-such-route", "", nil)

			resp, _ := execRequestRaw(ociClient, req)

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

			after := scrapeMetrics(metricsClient)
			afterCounter, ok := utils.FindCounter(after[requestsTotalMetric], wantLabels)
			Expect(ok).To(BeTrue())
			Expect(afterCounter - beforeCounter).To(Equal(float64(1)))

			// Invariant: nothing should emit an empty endpoint label. The
			// fallback in metricsmw.endpointLabel guarantees this for
			// missing bag values; this check catches regressions where a
			// new code path forgets to call SetEndpoint.
			for _, metric := range after[requestsTotalMetric].GetMetric() {
				Expect(labelValue(metric.GetLabel(), domain.LabelEndpoint)).
					NotTo(BeEmpty(), "counter series has empty endpoint label: %v", metric.GetLabel())
			}
		})
	})
})

func scrapeMetrics(client *http.Client) map[string]*dto.MetricFamily {
	_, body := scrapeMetricsBody(client)

	return utils.ParseMetricsText(body)
}

func scrapeMetricsBody(client *http.Client) (*http.Response, []byte) {
	resp, err := client.Get(metricsAddr + "/metrics")
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, body
}

func expectOK(client *http.Client, url string) {
	resp, err := client.Get(url)
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

func counterSumForEndpoints(fam *dto.MetricFamily, endpoints map[string]struct{}) float64 {
	var sum float64

	for _, metric := range fam.GetMetric() {
		if _, ok := endpoints[labelValue(metric.GetLabel(), domain.LabelEndpoint)]; ok {
			sum += metric.GetCounter().GetValue()
		}
	}

	return sum
}

func histogramCountSumForEndpoints(fam *dto.MetricFamily, endpoints map[string]struct{}) uint64 {
	var sum uint64

	for _, metric := range fam.GetMetric() {
		if _, ok := endpoints[labelValue(metric.GetLabel(), domain.LabelEndpoint)]; ok {
			sum += metric.GetHistogram().GetSampleCount()
		}
	}

	return sum
}

func labelValue(labels []*dto.LabelPair, name string) string {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue()
		}
	}

	return ""
}
