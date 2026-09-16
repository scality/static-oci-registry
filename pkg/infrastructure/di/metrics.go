package di

import "github.com/prometheus/client_golang/prometheus"

func (c *Container) getMetricsRegistry() *prometheus.Registry {
	if c.metricsRegistry == nil {
		c.metricsRegistry = prometheus.NewRegistry()
	}

	return c.metricsRegistry
}
