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

// wrapMetricsHandlerWithAuth guards /metrics access with Kubernetes auth.
// Validates bearer tokens and checks permissions via Kubernetes APIs
// (TokenReview, SubjectAccessReview). Applied only when METRICS_SECURE=true;
// startup failures are fatal.
func (c *Container) wrapMetricsHandlerWithAuth(next http.Handler) http.Handler {
	restConfig, err := c.getMetricsRESTConfig()
	if err != nil {
		c.GetLogger().ErrorContext(c.ctx, "failed to build kubernetes REST config for metrics auth",
			slog.Any("error", err),
		)
		os.Exit(1) //nolint:revive // startup misconfiguration is fatal
	}

	// Must build from restConfig, not DefaultClient.
	// DefaultClient loses the kubeconfig's TLS CA, breaking TokenReview/SubjectAccessReview auth.
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

// getMetricsRESTConfig loads Kubernetes config: tries METRICS_KUBECONFIG
// first, falls back to in-cluster config. Useful for local development.
func (c *Container) getMetricsRESTConfig() (*rest.Config, error) {
	if path := c.config.Metrics.Kubeconfig; path != "" {
		return clientcmd.BuildConfigFromFlags("", path) //nolint:wrapcheck // returned to a fatal callsite
	}

	return rest.InClusterConfig() //nolint:wrapcheck // returned to a fatal callsite
}
