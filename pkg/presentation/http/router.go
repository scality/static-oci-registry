package http

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

// Route is an HTTP handler that knows which paths it can serve and which
// methods are valid on those paths.
type Route interface {
	http.Handler
	Matches(path string) bool
	// AllowedMethods returns the HTTP methods this route accepts. A nil or
	// empty slice means "no method is allowed" — the dispatcher will then
	// always reject the request as UNSUPPORTED. Routes are expected to
	// advertise every method they handle (e.g. GET implies HEAD only if
	// the handler actually implements it).
	AllowedMethods() []string
}

// NewV2Router returns the dispatcher for the OCI /v2/ surface. It serves the
// version-check endpoint at /v2/ (end-1 of the spec) and otherwise dispatches
// to the first route whose Matches reports true. If a route matches the path
// but not the method, it returns an OCI error envelope using the UNSUPPORTED
// code via HandleError (404). If no route matches, it returns 404.
// Routes are evaluated in registration order.
func NewV2Router(logger *slog.Logger, routes ...Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// end-1: OCI version check
		if path == "/v2/" {
			w.WriteHeader(http.StatusOK)

			return
		}

		for _, route := range routes {
			if !route.Matches(path) {
				continue
			}

			if !slices.Contains(route.AllowedMethods(), r.Method) {
				// this will return 404 instead of 405, which is intentional
				HandleError(r.Context(), w, unsupportedMethodError(r.Method, route.AllowedMethods()), logger)

				return
			}

			route.ServeHTTP(w, r)

			return
		}

		// Unknown path under /v2/: respond 404. The OCI distribution-spec
		// does not define a generic error code for unknown endpoints
		// (UNSUPPORTED is reserved for known endpoints that aren't
		// implemented), and every major registry we checked (Docker Hub,
		// Quay, MCR, GHCR) returns 404 here, so we do the same.
		http.NotFound(w, r)
	})
}

// unsupportedMethodError builds an error with the OCI UNSUPPORTED code so
// HandleError can render it through the standard error envelope path. The
// conformance suite accepts 400 or 404 for write methods on read-only
// endpoints, and HandleError currently emits 404 for all OCI errors.
func unsupportedMethodError(method string, allowed []string) error {
	return errors.Wrap(
		errors.New("method "+method+" not allowed on this endpoint"),
		ocierrors.BuildOCIProperties(
			ocierrors.Unsupported,
			"method not allowed for this endpoint",
			map[string]string{
				"method":  method,
				"allowed": strings.Join(allowed, ", "),
			},
		),
	)
}
