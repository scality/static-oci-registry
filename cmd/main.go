package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"static-oci-registry/cmd/config"
	"static-oci-registry/pkg/infrastructure/di"
	"syscall"
	"time"
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
		logger.Info().Msg("http server starting")

		serveErr := httpServer.ListenAndServe()
		if serveErr != nil {
			sigCh <- syscall.SIGTERM

			// ErrServerClosed is returned on graceful close so we want to ignore that
			if !errors.Is(serveErr, http.ErrServerClosed) {
				logger.Error().Err(serveErr).Msg("Error serving http")
			}
		}

		logger.Info().Msg("http server stopped")
	}()

	// wait for anything to signal server termination
	<-sigCh

	err = httpServer.Shutdown(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Error shutting down http server")
	}

	logger.Info().Msg("service stopped")
}
