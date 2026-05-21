package di

import (
	"crypto/tls"
	"log/slog"
	"os"
)

func (c *Container) getTLSConfig() *tls.Config {
	if c.TLSConfig == nil {
		if c.config.HTTP.TLS.CertFilePath == "" || c.config.HTTP.TLS.KeyFilePath == "" {
			c.GetLogger().ErrorContext(c.ctx, "TLS certificate and key file paths are required",
				slog.String("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath),
				slog.String("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		serverCert, err := tls.LoadX509KeyPair(
			c.config.HTTP.TLS.CertFilePath,
			c.config.HTTP.TLS.KeyFilePath,
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to load server certificate and key",
				slog.String("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath),
				slog.String("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath),
				slog.Any("error_message", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	return c.TLSConfig
}
