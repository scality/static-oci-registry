package di

import (
	"net/http"
	"strings"
)

// getRouter returns the main HTTP router with all endpoints configured
func (c *Container) getRouter() http.Handler {
	if c.router == nil {
		router := http.NewServeMux()

		// Healthcheck endpoint for liveness status
		router.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// Healthcheck endpoint for kubernetes startup probe
		router.HandleFunc("/readyz", func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})

		// OCI endpoints under /v2/
		// Use prefix pattern to support multi-level image names with slashes
		router.Handle("/v2/", c.getV2Router())

		c.router = router
	}

	return c.router
}

// getV2Router returns a handler that routes /v2/ endpoints
// This supports multi-level image names with unencoded slashes.
func (c *Container) getV2Router() http.Handler {
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
