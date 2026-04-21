package di

import (
	"crypto/tls"
)

func (c *Container) getTLSConfig() *tls.Config {
	if c.TLSConfig == nil {
		if c.config.HTTP.TLS.CertFilePath == "" || c.config.HTTP.TLS.KeyFilePath == "" {
			c.GetLogger().Fatal().
				Str("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath).
				Str("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath).
				Msg("TLS certificate and key file paths are required")
		}

		serverCert, err := tls.LoadX509KeyPair(
			c.config.HTTP.TLS.CertFilePath,
			c.config.HTTP.TLS.KeyFilePath,
		)
		if err != nil {
			c.GetLogger().Fatal().Err(err).
				Str("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath).
				Str("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath).
				Msg("failed to load server certificate and key")
		}

		c.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	return c.TLSConfig
}
