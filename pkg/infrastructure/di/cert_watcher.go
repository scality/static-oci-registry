package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/certwatcher"
)

// GetCertWatcher returns a certwatcher.CertWatcher that watches the configured
// TLS certificate and key files and reloads them on change. Call Start on the
// returned watcher to begin watching for renewals.
func (c *Container) GetCertWatcher() *certwatcher.CertWatcher {
	if c.certWatcher == nil {
		if c.config.HTTP.TLS.CertFilePath == "" || c.config.HTTP.TLS.KeyFilePath == "" {
			c.GetLogger().ErrorContext(c.ctx, "TLS certificate and key file paths are required",
				slog.String("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath),
				slog.String("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		watcher, err := certwatcher.New(
			c.GetLogger(),
			c.config.HTTP.TLS.CertFilePath,
			c.config.HTTP.TLS.KeyFilePath,
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create TLS certificate watcher",
				slog.String("http_tls_cert_file_path", c.config.HTTP.TLS.CertFilePath),
				slog.String("http_tls_key_file_path", c.config.HTTP.TLS.KeyFilePath),
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.certWatcher = watcher
	}

	return c.certWatcher
}
