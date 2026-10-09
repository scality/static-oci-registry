package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/presentation/http/metricsmw"
)

// RequestMetrics groups the two vecs the HTTP middleware needs. Constructed
// once at startup and shared for the process lifetime.
//
// Tests wanting isolation build fresh vecs via NewRequestsCounter / NewRequestDuration
// and register them on their own prometheus.Registry instead.
type RequestMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// GetRequestCounter implements metricsmw.MetricsCollector.
func (m *RequestMetrics) GetRequestCounter() *prometheus.CounterVec {
	return m.Requests
}

// GetDurationHistogram implements metricsmw.MetricsCollector.
func (m *RequestMetrics) GetDurationHistogram() *prometheus.HistogramVec {
	return m.Duration
}

var _ metricsmw.MetricsCollector = (*RequestMetrics)(nil)

// NewRequestsCounter builds a fresh, unregistered requests_total counter vec
// carrying the standard label set. Callers own registration.
func NewRequestsCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: domain.MetricsNamespace,
			Subsystem: domain.MetricsSubsystem,
			Name:      "requests_total",
			Help: "Total number of HTTP requests served by the OCI registry, " +
				"labelled by method, response code, OCI endpoint, resolved " +
				"solution, and registry name.",
		},
		domain.MetricsLabels,
	)
}

// NewRequestDuration builds a fresh, unregistered request_duration_seconds
// histogram vec carrying the standard label set. Callers own registration.
func NewRequestDuration() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: domain.MetricsNamespace,
			Subsystem: domain.MetricsSubsystem,
			Name:      "request_duration_seconds",
			Help: "Duration of HTTP requests served by the OCI registry in seconds, " +
				"labelled by method, response code, OCI endpoint, resolved solution, " +
				"and registry name.",
			Buckets: domain.RequestDurationBuckets,
		},
		domain.MetricsLabels,
	)
}
