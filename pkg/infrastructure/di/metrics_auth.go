package di

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
)

// wrapMetricsHandlerWithAuth returns next wrapped with the kubebuilder-style
// authn/authz filter: bearer tokens are validated via TokenReview and callers
// must be authorized (via SubjectAccessReview) to `get` the non-resource URL
// `/metrics`. This is the same filter modern kubebuilder projects scaffold.
//
// The filter is only applied when METRICS_SECURE=true, so bearer tokens are
// never accepted over an unencrypted listener. A failure to construct the
// filter (bad kubeconfig, unreachable apiserver at startup, missing RBAC on
// the app's own ServiceAccount) is treated as fatal.
func (c *Container) wrapMetricsHandlerWithAuth(next http.Handler) http.Handler {
	restConfig, err := c.getMetricsRESTConfig()
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to build kubernetes REST config for metrics auth",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	// Build the http client from restConfig so its transport honours the
	// kubeconfig's TLS settings (CAData / CAFile). Passing nil here would let
	// client-go fall back to http.DefaultClient, which uses the system trust
	// store and drops the pod / kubeconfig CA on the floor — TokenReview and
	// SubjectAccessReview calls would then fail the TLS handshake and the
	// filter would map that transport error to HTTP 500 for every request.
	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to build kubernetes http client for metrics auth",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	filter, err := filters.WithAuthenticationAndAuthorization(restConfig, httpClient)
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to build metrics authn/authz filter",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	authHandler, err := filter(logr.FromSlogHandler(c.GetLogger().Handler()), next)
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to wrap metrics handler with authn/authz filter",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	return authHandler
}

// getMetricsRESTConfig returns a *rest.Config for the metrics auth filter.
// When METRICS_KUBECONFIG is set, it is loaded from that path (useful for
// local development). Otherwise the in-cluster config is used.
func (c *Container) getMetricsRESTConfig() (*rest.Config, error) {
	if path := c.config.Metrics.Kubeconfig; path != "" {
		return clientcmd.BuildConfigFromFlags("", path) //nolint:wrapcheck // returned to a fatal callsite
	}

	return rest.InClusterConfig() //nolint:wrapcheck // returned to a fatal callsite
}
