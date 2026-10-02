package di

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/static-oci-registry/pkg/domain"
)

// getMetricsRegistry returns the process-wide Prometheus registry. Lazily
// created so DI stays sequential and cheap when metrics are disabled.
func (c *Container) getMetricsRegistry() *prometheus.Registry {
	if c.metricsRegistry == nil {
		c.metricsRegistry = prometheus.NewRegistry()

		// register the metrics here
		c.metricsRegistry.MustRegister(
			domain.RegistryRequestMetrics.Requests,
			domain.RegistryRequestMetrics.Duration,
		)
	}

	return c.metricsRegistry
}
