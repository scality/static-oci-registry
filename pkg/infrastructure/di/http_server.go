package di

import (
	"net/http"
	"time"
)

const timeoutDurationInSeconds = 5

func (c *Container) GetHTTPServer() *http.Server {
	if c.httpServer == nil {
		router := http.NewServeMux()

		// Healthcheck endpoint for liveness status
		router.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// Healthcheck endpoint for kubernetes startup probe
		router.HandleFunc("/readyz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// OCI healthcheck endpoint (end-1 of the spec)
		router.HandleFunc("/v2/", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		c.httpServer = &http.Server{
			Addr:              c.config.HTTP.Addr,
			Handler:           router,
			ReadHeaderTimeout: timeoutDurationInSeconds * time.Second,
		}
	}

	return c.httpServer
}
