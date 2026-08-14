package di

import (
	"crypto/tls"
)

func (c *Container) getTLSConfig() *tls.Config {
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{
			// GetCertificate is resolved per-handshake by the cert watcher so
			// renewed certificates are picked up without restarting the server.
			GetCertificate: c.GetCertWatcher().GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	}

	return c.TLSConfig
}
