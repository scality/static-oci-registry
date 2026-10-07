package di

import (
	"net/http"

	apphttp "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/presentation/http/metricsmw"
)

// getHTTPRouter returns the main HTTP router with all endpoints configured.
// The OCI /v2/ sub-tree is wrapped by the metrics middleware so counter and
// duration observations carry the OCI endpoint and resolved solution labels.
// The health endpoints (/healthz, /readyz) are intentionally left
// uninstrumented — probe traffic would drown out real request signal.
func (c *Container) getHTTPRouter() http.Handler {
	if c.router == nil {
		httpRouter := http.NewServeMux()

		// Healthcheck endpoint for liveness status
		httpRouter.HandleFunc("/healthz", okHandler)

		// Healthcheck endpoint for kubernetes startup probe
		httpRouter.HandleFunc("/readyz", okHandler)

		// OCI endpoints under /v2/
		v2Router := apphttp.NewV2Router(
			c.GetLogger(),
			// UnsupportedEndpoint is registered first so any future blob
			// route additions cannot accidentally shadow the /blobs/uploads/
			// rejection path.
			c.getUnsupportedHandler(),
			c.getListTagsHandler(),
			c.getFetchManifestHandler(),
			c.getPullBlobHandler(),
		)

		httpRouter.Handle("/v2/", metricsmw.Wrap(
			v2Router,
			c.GetRequestMetrics(),
			c.config.Registry.Name,
		))

		c.router = httpRouter
	}

	return c.router
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
