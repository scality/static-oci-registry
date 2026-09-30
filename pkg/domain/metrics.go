package domain

import "github.com/prometheus/client_golang/prometheus"

const (
	namespace = "registry"
	subsystem = "http"

	LabelMethod          = "method"
	LabelCode            = "code"
	LabelEndpoint        = "endpoint"
	LabelSolutionName    = "solution_name"
	LabelSolutionVersion = "solution_version"
	LabelRegistry        = "registry"

	// EndpointUnknown is the value written to the `endpoint` label when a /v2/
	// request matches no route. Emitting a fixed sentinel (instead of the empty
	// string) keeps the observation countable while bounding cardinality: every
	// unmatched path folds into a single series rather than one per URL.
	EndpointUnknown = "unknown"
)

// requestDurationBuckets are tuned for a registry serving blobs whose sizes
// span kilobyte manifests to multi-gigabyte layers. The tail extends to 30
// minutes so long pulls do not silently fall into the +Inf bucket.
//
//nolint:gochecknoglobals // fixed histogram layout is intentional
var requestDurationBuckets = []float64{
	0.005, 0.025, 0.1, 0.25, 1, 5, 30, 120, 600, 1800,
}

// Common label set carried by both request vecs. Order is not significant to
// Prometheus but the shared slice keeps counter and histogram in lockstep.
//
//nolint:gochecknoglobals // fixed label schema is intentional
var labels = []string{
	LabelMethod,
	LabelCode,
	LabelEndpoint,
	LabelSolutionName,
	LabelSolutionVersion,
	LabelRegistry,
}

// RequestMetrics groups the two vecs the HTTP middleware needs. Constructed
// once at startup and shared for the process lifetime.
type RequestMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// RegistryRequestMetrics is the process-wide singleton wired into the HTTP
// middleware at DI init. Tests wanting isolation build fresh vecs via
// NewRequestsCounter / NewRequestDuration and register them on their own
// prometheus.Registry instead.
//
//nolint:gochecknoglobals // singleton by design; see factories for a per-registry alternative
var RegistryRequestMetrics = RequestMetrics{
	Requests: NewRequestsCounter(),
	Duration: NewRequestDuration(),
}

// NewRequestsCounter builds a fresh, unregistered requests_total counter vec
// carrying the standard label set. Callers own registration.
func NewRequestsCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
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
}

// NewRequestDuration builds a fresh, unregistered request_duration_seconds
// histogram vec carrying the standard label set. Callers own registration.
func NewRequestDuration() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
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
}
