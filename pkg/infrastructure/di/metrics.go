package di

import (
	"log/slog"
	"os"

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

// getRegistryName resolves the value of the `registry` HTTP-metric label. It
// prefers the REGISTRY_NAME env var; when empty it falls back to os.Hostname()
// so operators get a useful default without configuration; if hostname lookup
// also fails, it logs a warning and returns "" (Prometheus accepts empty
// label values).
func (c *Container) getRegistryName() string {
	if c.registryName != "" {
		return c.registryName
	}

	if name := c.config.Registry.Name; name != "" {
		c.registryName = name

		return c.registryName
	}

	host, err := os.Hostname()
	if err != nil {
		c.GetLogger().WarnContext(c.ctx,
			"failed to resolve hostname for registry metric label; using empty string",
			slog.Any("error", err),
		)

		return ""
	}

	c.registryName = host

	return c.registryName
}
