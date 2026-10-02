package di

import (
	"net/http"

	"github.com/scality/static-oci-registry/pkg/presentation/http/handler"
	"github.com/scality/static-oci-registry/pkg/presentation/metrics"
)

func (c *Container) getListTagsHandler() *handler.ListTags {
	if c.listTagsHandler == nil {
		c.listTagsHandler = handler.NewListTags(c.getListTagsUseCase(), c.GetLogger())
	}

	return c.listTagsHandler
}

func (c *Container) getFetchManifestHandler() *handler.FetchManifest {
	if c.fetchManifestHandler == nil {
		c.fetchManifestHandler = handler.NewFetchManifest(
			c.GetLogger(),
			c.getFetchManifestFromTagUseCase(),
			c.getFetchManifestFromDigestUseCase(),
		)
	}

	return c.fetchManifestHandler
}

func (c *Container) getPullBlobHandler() *handler.PullBlob {
	if c.pullBlobHandler == nil {
		c.pullBlobHandler = handler.NewPullBlob(c.GetLogger(), c.getPullBlobUseCase())
	}

	return c.pullBlobHandler
}

func (c *Container) getUnsupportedHandler() *handler.UnsupportedEndpoint {
	if c.unsupportedHandler == nil {
		c.unsupportedHandler = handler.NewUnsupportedEndpoint()
	}

	return c.unsupportedHandler
}

func (c *Container) getMetricsHandler() http.Handler {
	if c.metricsHandler == nil {
		h := metrics.NewHandler(
			c.GetLogger(),
			c.getMetricsRegistry(),
		)

		if c.config.Metrics.Secure {
			h = c.wrapMetricsHandlerWithAuth(h)
		}

		c.metricsHandler = h
	}

	return c.metricsHandler
}
