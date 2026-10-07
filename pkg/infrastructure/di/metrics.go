package di

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/static-oci-registry/pkg/infrastructure/metrics"
)

// GetRequestMetrics returns the process-wide HTTP request metrics singleton.
func (c *Container) GetRequestMetrics() *metrics.RequestMetrics {
	if c.requestMetrics == nil {
		c.requestMetrics = &metrics.RequestMetrics{
			Requests: metrics.NewRequestsCounter(),
			Duration: metrics.NewRequestDuration(),
		}
	}

	return c.requestMetrics
}

// getMetricsRegistry returns the process-wide Prometheus registry. Lazily
// created so DI stays sequential and cheap when metrics are disabled.
func (c *Container) getMetricsRegistry() *prometheus.Registry {
	if c.metricsRegistry == nil {
		c.metricsRegistry = prometheus.NewRegistry()

		// register the metrics here
		m := c.GetRequestMetrics()
		c.metricsRegistry.MustRegister(
			m.Requests,
			m.Duration,
		)
	}

	return c.metricsRegistry
}
