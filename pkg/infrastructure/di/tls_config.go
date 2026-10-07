package di

import (
	"crypto/tls"
)

func (c *Container) getTLSConfig() *tls.Config {
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{
			// GetCertificate is resolved per-handshake by the cert watcher so
			// renewed certificates are picked up without restarting the server.
			GetCertificate: c.GetHTTPCertWatcher().GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	}

	return c.TLSConfig
}

// getMetricsTLSConfig returns the TLS config for the metrics HTTP server, or
// nil when the metrics endpoint is configured to run without TLS
// (METRICS_SECURE=false). A nil TLSConfig signals startHTTPServer to serve
// plain HTTP.
func (c *Container) getMetricsTLSConfig() *tls.Config {
	if !c.config.Metrics.Secure {
		return nil
	}

	if c.metricsTLSConfig == nil {
		c.metricsTLSConfig = &tls.Config{
			// GetCertificate is resolved per-handshake by the cert watcher so
			// renewed certificates are picked up without restarting the server.
			GetCertificate: c.GetMetricsCertWatcher().GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	}

	return c.metricsTLSConfig
}
