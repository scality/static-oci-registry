package di

import "github.com/scality/static-oci-registry/pkg/presentation/http/handler"

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
