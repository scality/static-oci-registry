package http

import "net/http"

// Route is an HTTP handler that knows which paths it can serve.
type Route interface {
	http.Handler
	Matches(path string) bool
}

// NewV2Router returns the dispatcher for the OCI /v2/ surface. It serves the
// version-check endpoint at /v2/ (end-1 of the spec) and otherwise dispatches
// to the first route whose Matches reports true; if none match, it returns
// 404 Not Found. Routes are evaluated in registration order.
func NewV2Router(routes ...Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// end-1: OCI version check
		if path == "/v2/" {
			w.WriteHeader(http.StatusOK)

			return
		}

		for _, route := range routes {
			if route.Matches(path) {
				route.ServeHTTP(w, r)

				return
			}
		}

		// Unknown path under /v2/: respond 404. The OCI distribution-spec
		// does not define a generic error code for unknown endpoints
		// (UNSUPPORTED is reserved for known endpoints that aren't
		// implemented), and every major registry we checked (Docker Hub,
		// Quay, MCR, GHCR) returns 404 here, so we do the same.
		http.NotFound(w, r)
	})
}
