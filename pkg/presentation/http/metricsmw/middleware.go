// Package metricsmw wraps an http.Handler with a request-scoped label bag and
// the two Prometheus vecs defined in request_metrics.go. It composes promhttp's
// counter and duration instrumenters so status-code and method labels are
// captured automatically, and reads the endpoint/solution labels from the
// reqlabels bag at end-of-request via WithLabelFromCtx.
package metricsmw

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/scality/static-oci-registry/pkg/presentation/http/reqlabels"
)

// Wrap returns next wrapped with:
//  1. A request-scoped reqlabels.RequestLabels bag installed on the request
//     context so the router and handlers can mutate endpoint/solution labels.
//  2. promhttp.InstrumentHandlerCounter and InstrumentHandlerDuration reading
//     the bag via WithLabelFromCtx to observe the request when it completes.
//
// The `registry` label is curried into the vecs at construction so the fully
// labelled child vec is fixed for the lifetime of the process and no dynamic
// registry-value plumbing is needed.
func Wrap(next http.Handler, m *RequestMetrics, registry string) http.Handler {
	registryLabels := prometheus.Labels{LabelRegistry: registry}

	// MustCurryWith panics only if `registry` is not a defined label on the
	// vec, which is a programming error caught by unit tests.
	counter := m.Requests.MustCurryWith(registryLabels)
	duration := m.Duration.MustCurryWith(registryLabels)

	endpointFromCtx := promhttp.WithLabelFromCtx(LabelEndpoint, endpointLabel)
	solutionNameFromCtx := promhttp.WithLabelFromCtx(
		LabelSolutionName, solutionNameLabel,
	)
	solutionVersionFromCtx := promhttp.WithLabelFromCtx(
		LabelSolutionVersion, solutionVersionLabel,
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
	return EndpointUnknown
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
