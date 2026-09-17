// request_metrics.go owns the Prometheus vec definitions and their registration.
// It sits alongside middleware.go because the middleware is the sole consumer
// and no other layer needs to see RequestMetrics.
package metricsmw

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/go-errors"
)

// Metric name components. Kept as constants so the middleware layer can
// reference them if needed (for tests) without hard-coding string literals.
const (
	namespace = "registry"
	subsystem = "http"
)

// Label names used by the two request-scoped vecs. Order does not matter for
// correctness but is fixed here so tests can construct expected label sets.
const (
	LabelMethod          = "method"
	LabelCode            = "code"
	LabelEndpoint        = "endpoint"
	LabelSolutionName    = "solution_name"
	LabelSolutionVersion = "solution_version"
	LabelRegistry        = "registry"
)

// EndpointUnknown is the value written to the `endpoint` label when a /v2/
// request matches no route. Emitting a fixed sentinel (instead of the empty
// string) keeps the observation countable while bounding cardinality: every
// unmatched path folds into a single series rather than one per URL.
const EndpointUnknown = "unknown"

// requestDurationBuckets are tuned for a registry serving blobs whose sizes
// span kilobyte manifests to multi-gigabyte layers. The tail extends to 30
// minutes so long pulls do not silently fall into the +Inf bucket.
//
//nolint:gochecknoglobals // fixed histogram layout is intentional
var requestDurationBuckets = []float64{
	0.005, 0.025, 0.1, 0.25, 1, 5, 30, 120, 600, 1800,
}

// RequestMetrics groups the two vecs the HTTP middleware needs. Constructed
// once at startup and shared for the process lifetime.
type RequestMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// NewRequestMetrics builds the two vecs and registers them with reg. It
// returns an error rather than panicking so the DI layer can decide how to
// react (the current convention is to os.Exit at startup).
func NewRequestMetrics(reg prometheus.Registerer) (*RequestMetrics, error) {
	labels := []string{
		LabelMethod,
		LabelCode,
		LabelEndpoint,
		LabelSolutionName,
		LabelSolutionVersion,
		LabelRegistry,
	}

	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "requests_total",
			Help: "Total number of HTTP requests served by the OCI registry, " +
				"labelled by method, response code, OCI endpoint, resolved " +
				"solution, and registry name.",
		},
		labels,
	)

	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help: "Duration of HTTP requests served by the OCI registry in seconds, " +
				"labelled by method, response code, OCI endpoint, resolved solution, " +
				"and registry name.",
			Buckets: requestDurationBuckets,
		},
		labels,
	)

	if err := reg.Register(requests); err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to register registry_http_requests_total"))
	}

	if err := reg.Register(duration); err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to register registry_http_request_duration_seconds"))
	}

	return &RequestMetrics{Requests: requests, Duration: duration}, nil
}
