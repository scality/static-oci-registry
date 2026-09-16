package metrics

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewHandler returns an http.Handler that exposes the Prometheus scrape
// endpoint for the given registry. Errors encountered while gathering or
// writing metrics are surfaced through the provided slog logger at error
// level, and reported to the client as HTTP 500.
func NewHandler(
	logger *slog.Logger,
	registry prometheus.Gatherer,
) http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
}
