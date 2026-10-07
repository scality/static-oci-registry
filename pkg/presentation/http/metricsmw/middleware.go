// Package metricsmw wires the HTTP handlers with Prometheus middlewares.
// Composes request counters and latency histograms, populated with endpoint/solution labels.
package metricsmw

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/metrics"
	"github.com/scality/static-oci-registry/pkg/presentation/http/reqlabels"
)

// Wrap `next` with request metrics.
// The label bag is installed on the request context for handlers to
// populate with endpoint/solution labels.
func Wrap(next http.Handler, m *metrics.RequestMetrics, registry string) http.Handler {
	registryLabels := prometheus.Labels{domain.LabelRegistry: registry}

	// MustCurryWith panics only if `registry` is not a defined label on the
	// vec, which is a programming error caught by unit tests.
	counter := m.Requests.MustCurryWith(registryLabels)
	duration := m.Duration.MustCurryWith(registryLabels)

	endpointFromCtx := promhttp.WithLabelFromCtx(domain.LabelEndpoint, endpointLabel)
	solutionNameFromCtx := promhttp.WithLabelFromCtx(
		domain.LabelSolutionName, solutionNameLabel,
	)
	solutionVersionFromCtx := promhttp.WithLabelFromCtx(
		domain.LabelSolutionVersion, solutionVersionLabel,
	)

	instrumented := promhttp.InstrumentHandlerCounter(
		counter,
		promhttp.InstrumentHandlerDuration(
			duration,
			next,
			endpointFromCtx, solutionNameFromCtx, solutionVersionFromCtx,
		),
		endpointFromCtx, solutionNameFromCtx, solutionVersionFromCtx,
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := reqlabels.NewContext(r.Context())
		instrumented.ServeHTTP(w, r.WithContext(ctx))
	})
}

func endpointLabel(ctx context.Context) string {
	if l := reqlabels.From(ctx); l != nil && l.Endpoint != "" {
		return l.Endpoint
	}

	// The bag stays empty when a /v2/ request matched no route. Folding all
	// such requests into a single "unknown" series keeps them countable
	// without letting attacker-controlled URLs blow up label cardinality.
	return domain.EndpointUnknown
}

func solutionNameLabel(ctx context.Context) string {
	if l := reqlabels.From(ctx); l != nil {
		return l.SolutionName
	}

	return ""
}

func solutionVersionLabel(ctx context.Context) string {
	if l := reqlabels.From(ctx); l != nil {
		return l.SolutionVersion
	}

	return ""
}
