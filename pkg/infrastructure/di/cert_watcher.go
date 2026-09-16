package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/certwatcher"
)

// GetHTTPCertWatcher returns a certwatcher.CertWatcher that watches the configured
// TLS certificate and key files and reloads them on change. Call Start on the
// returned watcher to begin watching for renewals.
func (c *Container) GetHTTPCertWatcher() *certwatcher.CertWatcher {
	if c.httpCertWatcher == nil {
		c.httpCertWatcher = c.newCertWatcher(
			"http",
			c.config.HTTP.TLS.CertFilePath,
			c.config.HTTP.TLS.KeyFilePath,
		)
	}

	return c.httpCertWatcher
}

// GetMetricsCertWatcher returns a certwatcher.CertWatcher that watches the
// configured TLS certificate and key files and reloads them on change. It
// returns nil when the metrics endpoint is disabled (METRICS_ADDR="0") or
// when metrics is configured to run without TLS (METRICS_SECURE=false), in
// which case no cert watcher is needed. Call Start on the returned watcher
// to begin watching for renewals.
func (c *Container) GetMetricsCertWatcher() *certwatcher.CertWatcher {
	if c.config.Metrics.Addr == "0" || !c.config.Metrics.Secure {
		return nil
	}

	if c.metricsCertWatcher == nil {
		c.metricsCertWatcher = c.newCertWatcher(
			"metrics",
			c.config.Metrics.TLS.CertFilePath,
			c.config.Metrics.TLS.KeyFilePath,
		)
	}

	return c.metricsCertWatcher
}

func (c *Container) newCertWatcher(label, certPath, keyPath string) *certwatcher.CertWatcher {
	if certPath == "" || keyPath == "" {
		c.GetLogger().ErrorContext(c.ctx, "TLS certificate and key file paths are required",
			slog.String("component", label),
			slog.String("cert_file_path", certPath),
			slog.String("key_file_path", keyPath),
		)
		os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
	}

	watcher, err := certwatcher.New(c.GetLogger(), certPath, keyPath)
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to create TLS certificate watcher",
			slog.String("component", label),
			slog.String("cert_file_path", certPath),
			slog.String("key_file_path", keyPath),
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
	}

	return watcher
}
