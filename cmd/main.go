package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	go startCertWatcher(ctx, logger, container.GetCertWatcher(), sigCh)
	go startHTTPServer(ctx, logger, httpServer, sigCh)

	// wait for anything to signal server termination
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, shutdownTimeout)
	defer shutdownCancel()

	err = httpServer.Shutdown(shutdownCtx)
	if err != nil {
		logger.ErrorContext(ctx, "Error shutting down http server",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "service stopped")
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
func startHTTPServer(
	ctx context.Context,
	logger *slog.Logger,
	httpServer *http.Server,
	sigCh chan<- os.Signal,
) {
	logger.InfoContext(ctx, "http server starting")

	serveErr := httpServer.ListenAndServeTLS("", "")
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
