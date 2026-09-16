package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/certwatcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
)

// shutdownTimeout bounds how long the server is given to drain in-flight
// connections on graceful shutdown.
const shutdownTimeout = 5 * time.Second

func main() {
	log.Printf("Starting %s version %s", config.ApplicationName, config.ApplicationVersion)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.NewEnvironment(ctx)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	container := di.NewContainer(ctx, cfg)

	logger := container.GetLogger()

	// handle Shutdown signals from the OS
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	httpServer := container.GetHTTPServer()

	httpLogger := logger.With(slog.String("component", "http"))
	go startCertWatcher(ctx, httpLogger, container.GetHTTPCertWatcher(), sigCh)
	go startHTTPServer(ctx, httpLogger, httpServer, sigCh)

	metricsServer := container.GetMetricsServer()
	spawnMetricsGoroutines(ctx, logger, container, sigCh)

	// wait for anything to signal server termination
	<-sigCh

	if shutdownServers(ctx, logger, httpServer, metricsServer) {
		os.Exit(1)
	}

	logger.InfoContext(ctx, "service stopped")
}

// spawnMetricsGoroutines starts the metrics HTTP server and its cert watcher
// when metrics are enabled, respecting METRICS_SECURE for the TLS toggle.
func spawnMetricsGoroutines(
	ctx context.Context,
	logger *slog.Logger,
	container *di.Container,
	sigCh chan<- os.Signal,
) {
	metricsLogger := logger.With(slog.String("component", "metrics"))
	metricsServer := container.GetMetricsServer()
	metricsCertWatcher := container.GetMetricsCertWatcher()

	switch {
	case metricsServer == nil:
		metricsLogger.InfoContext(ctx, "metrics http server is disabled",
			slog.String("reason", `METRICS_ADDR is "0"`),
		)
	case metricsCertWatcher == nil:
		metricsLogger.WarnContext(ctx, "metrics http server is running without TLS",
			slog.String("reason", "METRICS_SECURE is false"),
		)

		go startHTTPServer(ctx, metricsLogger, metricsServer, sigCh)
	default:
		go startCertWatcher(ctx, metricsLogger, metricsCertWatcher, sigCh)
		go startHTTPServer(ctx, metricsLogger, metricsServer, sigCh)
	}
}

// shutdownServers drains both HTTP servers in parallel with a bounded timeout
// and returns true when either server failed to close cleanly.
func shutdownServers(
	ctx context.Context,
	logger *slog.Logger,
	httpServer *http.Server,
	metricsServer *http.Server,
) bool {
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, shutdownTimeout)
	defer shutdownCancel()

	var (
		wg         sync.WaitGroup
		httpErr    error
		metricsErr error
	)

	wg.Go(func() {
		httpErr = httpServer.Shutdown(shutdownCtx)
		if httpErr != nil {
			logger.ErrorContext(ctx, "Error shutting down http server",
				slog.Any("error", httpErr),
			)
		}
	})

	if metricsServer != nil {
		wg.Go(func() {
			metricsErr = metricsServer.Shutdown(shutdownCtx)
			if metricsErr != nil {
				logger.ErrorContext(ctx, "Error shutting down metrics server",
					slog.Any("error", metricsErr),
				)
			}
		})
	}

	wg.Wait()

	return httpErr != nil || metricsErr != nil
}

// startCertWatcher runs the TLS certificate watcher, signalling shutdown if it
// stops with an error.
func startCertWatcher(
	ctx context.Context,
	logger *slog.Logger,
	certWatcher *certwatcher.CertWatcher,
	sigCh chan<- os.Signal,
) {
	logger.InfoContext(ctx, "tls certificate watcher starting")

	if watchErr := certWatcher.Start(ctx); watchErr != nil {
		logger.ErrorContext(ctx, "tls certificate watcher stopped",
			slog.Any("error", watchErr),
		)

		signalShutdown(sigCh)
	}
}

// signalShutdown performs a non-blocking send on sigCh. The channel is buffered
// and only the first signal matters, so if a shutdown is already pending we drop
// this one rather than blocking the goroutine forever.
func signalShutdown(sigCh chan<- os.Signal) {
	select {
	case sigCh <- syscall.SIGTERM:
	default:
	}
}

// startHTTPServer runs the HTTP server, signalling shutdown when it stops.
// When the server's TLSConfig is nil the listener serves plain HTTP; otherwise
// certificates are resolved per-handshake by the associated cert watcher.
func startHTTPServer(
	ctx context.Context,
	logger *slog.Logger,
	httpServer *http.Server,
	sigCh chan<- os.Signal,
) {
	logger.InfoContext(ctx, "http server starting")

	var serveErr error
	if httpServer.TLSConfig != nil {
		serveErr = httpServer.ListenAndServeTLS("", "")
	} else {
		serveErr = httpServer.ListenAndServe()
	}
	if serveErr != nil {
		signalShutdown(sigCh)

		// ErrServerClosed is returned on graceful close so we want to ignore that
		if !errors.Is(serveErr, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "Error serving http",
				slog.Any("error", serveErr),
			)
		}
	}

	logger.InfoContext(ctx, "http server stopped")
}
