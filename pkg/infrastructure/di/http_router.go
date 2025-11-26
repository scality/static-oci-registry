package di

import (
	"net/http"
	"strings"
)

// getHTTPRouter returns the main HTTP router with all endpoints configured.
func (c *Container) getHTTPRouter() http.Handler {
	if c.router == nil {
		httpRouter := http.NewServeMux()

		// Healthcheck endpoint for liveness status
		httpRouter.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// Healthcheck endpoint for kubernetes startup probe
		httpRouter.HandleFunc("/readyz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// OCI endpoints under /v2/
		// Use prefix pattern to support multi-level image names with slashes
		// This supports multi-level image names with unencoded slashes.
		getV2Router := func() http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path

				// OCI healthcheck endpoint (end-1 of the spec)
				if path == "/v2/" {
					w.WriteHeader(http.StatusOK)
					return
				}

				// OCI tag list endpoint (end-8a and 8b of the spec)
				if strings.HasSuffix(path, "/tags/list") {
					c.getListTagsHandler().ServeHTTP(w, r)
					return
				}

				// Unknown endpoint
				http.NotFound(w, r)
			})
		}
		httpRouter.Handle("/v2/", getV2Router())

		c.router = httpRouter
	}

	return c.router
}
