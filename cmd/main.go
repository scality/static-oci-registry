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
	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
)

const timeoutDurationInSeconds = 5

func main() {
	log.Printf("Starting %s version %s", config.ApplicationName, config.ApplicationVersion)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutDurationInSeconds*time.Second)
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

	go func() {
		logger.InfoContext(ctx, "http server starting")

		serveErr := httpServer.ListenAndServeTLS("", "")
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
	}()

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
