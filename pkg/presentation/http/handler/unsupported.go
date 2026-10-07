package handler

import (
	"net/http"
	"regexp"
)

// uploadsURLPattern matches the push/upload surface of the OCI distribution
// spec (end-4 through end-11). This is a read-only registry; declaring a
// dedicated route lets the dispatcher reject any method on these paths with
// 405 UNSUPPORTED instead of a generic 404, which is what most read-only
// registries do and what spec-conforming clients expect.
const uploadsURLPattern = `^/v2/.+/blobs/uploads(/.*)?$`

var uploadsURLRegex = regexp.MustCompile(uploadsURLPattern)

// UnsupportedEndpoint matches paths under the OCI push surface and refuses
// every method. The dispatcher fires the method check first, so ServeHTTP
// is unreachable in normal operation; it returns 405 defensively in case
// the route is ever wired up without the dispatcher.
type UnsupportedEndpoint struct{}

func NewUnsupportedEndpoint() *UnsupportedEndpoint {
	return &UnsupportedEndpoint{}
}

// Matches reports whether the request path falls under the push surface.
func (*UnsupportedEndpoint) Matches(path string) bool {
	return uploadsURLRegex.MatchString(path)
}

// AllowedMethods returns nil: no method is accepted on these paths.
func (*UnsupportedEndpoint) AllowedMethods() []string {
	return nil
}

// EndpointName returns the OCI-spec label of this route, used as the
// `endpoint` HTTP metric label. All push-surface endpoints (end-4..7,
// end-9..11) collapse under this single label because we reject them all
// with the same UNSUPPORTED code.
func (*UnsupportedEndpoint) EndpointName() string {
	return "unsupported"
}

// ServeHTTP is unreachable through the /v2/ dispatcher (the method check
// rejects every request before reaching here). It returns 405 as a
// fail-safe if the route is wired up directly.
func (*UnsupportedEndpoint) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}
