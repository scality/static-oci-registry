package di

import (
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/static-oci-registry/pkg/presentation/http/metricsmw"
)

// getMetricsRegistry returns the process-wide Prometheus registry. Lazily
// created so DI stays sequential and cheap when metrics are disabled.
func (c *Container) getMetricsRegistry() *prometheus.Registry {
	if c.metricsRegistry == nil {
		c.metricsRegistry = prometheus.NewRegistry()
	}

	return c.metricsRegistry
}

// getRequestMetrics builds and registers the HTTP request-scoped vecs
// (counter and duration histogram). Registration failures are treated as
// fatal because a duplicate registration is a programming error, not a
// runtime condition.
func (c *Container) getRequestMetrics() *metricsmw.RequestMetrics {
	if c.requestMetrics != nil {
		return c.requestMetrics
	}

	m, err := metricsmw.NewRequestMetrics(c.getMetricsRegistry())
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to register HTTP request metrics",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	c.requestMetrics = m

	return c.requestMetrics
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
