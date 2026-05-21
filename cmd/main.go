package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
)

const timeoutDurationInSeconds = 5

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDurationInSeconds*time.Second)
	defer cancel()

	cfg, err := config.NewEnvironment(ctx)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	container := di.NewContainer(ctx, cfg)

	logger := container.GetLogger()

	logger.InfoContext(ctx, "starting application")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	httpServer := container.GetHTTPServer()

	go runHTTPServer(ctx, logger, httpServer, sigCh)

	// wait for anything to signal server termination
	<-sigCh

	err = httpServer.Shutdown(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Error shutting down http server",
			slog.Any("error_message", err),
		)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "service stopped")
}

func runHTTPServer(
	ctx context.Context,
	logger *slog.Logger,
	srv *http.Server,
	sigCh chan<- os.Signal,
) {
	logger.InfoContext(ctx, "http server starting")

	serveErr := srv.ListenAndServeTLS("", "")
	if serveErr != nil {
		sigCh <- syscall.SIGTERM

		// ErrServerClosed is returned on graceful close so we want to ignore that
		if !errors.Is(serveErr, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "Error serving http",
				slog.Any("error_message", serveErr),
			)
		}
	}

	logger.InfoContext(ctx, "http server stopped")
}
