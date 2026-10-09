// Package reqlabels carries per-request labels between the router, the
// handlers, and the HTTP metrics middleware. A single *RequestLabels value is
// stashed in the request context by the middleware, mutated in place by the
// router (endpoint) and the handlers (solution/version), and read at the end
// of the request by promhttp's WithLabelFromCtx callbacks.
//
// Using a mutable bag rather than context.WithValue re-wrapping avoids having
// to thread a new context through every ServeHTTP call: the bag pointer is
// captured once and the router/handlers just assign to its fields.
package reqlabels

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

// RequestLabels holds the dynamic label values collected as an HTTP request
// flows through the router and its handlers. The endpoint label falls back to
// metricsmw.EndpointUnknown at observation time when this bag is empty
// (unknown /v2/ path); solution labels stay empty by design when unresolved.
type RequestLabels struct {
	// Endpoint is the OCI-spec label of the matched route (for example
	// "list_tags", "fetch_manifest", "pull_blob", "version_check",
	// "unsupported"). Left empty when no route matched; the metrics
	// middleware surfaces that as "unknown" so cardinality stays bounded.
	Endpoint string

	// SolutionName is the name of the solution whose layout served the
	// request. Empty means unresolved (for example ListTags aggregates
	// across all solutions, or the request failed before a layout was
	// picked).
	SolutionName string

	// SolutionVersion is the version of the solution whose layout served
	// the request. Empty means unresolved (see SolutionName).
	SolutionVersion string
}

// SetEndpoint records the endpoint label. Safe on a nil receiver so callers
// do not have to guard for missing middleware.
func (r *RequestLabels) SetEndpoint(name string) {
	if r == nil {
		return
	}

	r.Endpoint = name
}

// SetSolution records the solution/version label pair. Safe on a nil receiver
// so callers do not have to guard for missing middleware.
func (r *RequestLabels) SetSolution(sv domain.SolutionVersion) {
	if r == nil {
		return
	}

	r.SolutionName = sv.Solution
	r.SolutionVersion = sv.Version
}

type ctxKey struct{}

// NewContext returns a child context that carries a fresh RequestLabels bag,
// plus a pointer to that bag so the caller can pass it to downstream code
// that reads from ctx.
func NewContext(parent context.Context) (context.Context, *RequestLabels) {
	labels := &RequestLabels{}

	return context.WithValue(parent, ctxKey{}, labels), labels
}

// From returns the RequestLabels bag attached to ctx, or nil if none was
// installed. The Set* methods on RequestLabels are nil-safe so callers can
// dereference the result unconditionally.
func From(ctx context.Context) *RequestLabels {
	if ctx == nil {
		return nil
	}

	labels, ok := ctx.Value(ctxKey{}).(*RequestLabels)
	if !ok {
		return nil
	}

	return labels
}
