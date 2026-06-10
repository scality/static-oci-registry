package di

import "github.com/scality/static-oci-registry/pkg/usecase"

func (c *Container) getListTagsUseCase() *usecase.ListTags {
	if c.listTagsUseCase == nil {
		c.listTagsUseCase = usecase.NewListTags(
			c.GetLogger(),
			c.getTagLister(),
		)
	}

	return c.listTagsUseCase
}

func (c *Container) getFetchManifestFromTagUseCase() *usecase.FetchManifestFromTag {
	if c.fetchManifestFromTagUseCase == nil {
		c.fetchManifestFromTagUseCase = usecase.NewFetchManifestFromTag(
			c.GetLogger(),
			c.getTagManifestFetcher(),
		)
	}

	return c.fetchManifestFromTagUseCase
}

func (c *Container) getFetchManifestFromDigestUseCase() *usecase.FetchManifestFromDigest {
	if c.fetchManifestFromDigestUseCase == nil {
		c.fetchManifestFromDigestUseCase = usecase.NewFetchManifestFromDigest(
			c.GetLogger(),
			c.getDigestManifestFetcher(),
		)
	}

	return c.fetchManifestFromDigestUseCase
}

func (c *Container) getPullBlobUseCase() *usecase.PullBlob {
	if c.pullBlobUseCase == nil {
		c.pullBlobUseCase = usecase.NewPullBlob(
			c.GetLogger(),
			c.getBlobPuller(),
		)
	}

	return c.pullBlobUseCase
}
