package metricsmw

import "github.com/prometheus/client_golang/prometheus"

// MetricsCollector defines the interface for HTTP request metrics collection.
// Middleware depends on this interface, not on any infrastructure implementation.
type MetricsCollector interface {
	GetRequestCounter() *prometheus.CounterVec
	GetDurationHistogram() *prometheus.HistogramVec
}
