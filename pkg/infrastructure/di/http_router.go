package di

import (
	"net/http"

	apphttp "github.com/scality/static-oci-registry/pkg/presentation/http"
)

// getHTTPRouter returns the main HTTP router with all endpoints configured.
func (c *Container) getHTTPRouter() http.Handler {
	if c.router == nil {
		httpRouter := http.NewServeMux()

		// Healthcheck endpoint for liveness status
		httpRouter.HandleFunc("/healthz", okHandler)

		// Healthcheck endpoint for kubernetes startup probe
		httpRouter.HandleFunc("/readyz", okHandler)

		// OCI endpoints under /v2/
		httpRouter.Handle("/v2/", apphttp.NewV2Router(
			c.getListTagsHandler(),
			c.getFetchManifestHandler(),
		))

		c.router = httpRouter
	}

	return c.router
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
