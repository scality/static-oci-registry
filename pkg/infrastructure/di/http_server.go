package di

import (
	"net/http"
	"time"
)

const timeoutDurationInSeconds = 5

func (c *Container) GetHTTPServer() *http.Server {
	if c.httpServer == nil {
		c.httpServer = &http.Server{
			Addr:              c.config.HTTP.Addr,
			Handler:           c.getHTTPRouter(),
			ReadHeaderTimeout: timeoutDurationInSeconds * time.Second,
			TLSConfig:         c.getTLSConfig(),
		}
	}

	return c.httpServer
}

// GetMetricsServer returns the metrics HTTP server, or nil when the metrics
// endpoint is disabled (METRICS_ADDR="0", kubebuilder convention).
func (c *Container) GetMetricsServer() *http.Server {
	if c.config.Metrics.Addr == "0" {
		return nil
	}

	if c.metricsServer == nil {
		c.metricsServer = &http.Server{
			Addr:              c.config.Metrics.Addr,
			Handler:           c.getMetricsHandler(),
			ReadHeaderTimeout: timeoutDurationInSeconds * time.Second,
			TLSConfig:         c.getMetricsTLSConfig(),
		}
	}

	return c.metricsServer
}
