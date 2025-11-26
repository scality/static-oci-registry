package di

import (
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
)

func (c *Container) getTagLister() *taglister.FileSystem {
	if c.tagLister == nil {
		l, err := taglister.NewFileSystem(
			c.GetLogger(),
			c.getImageFinder(),
			c.config.FS.Root,
		)
		if err != nil {
			c.GetLogger().Fatal().Err(err).Msg("failed to create tag lister")
		}

		c.tagLister = l
	}

	return c.tagLister
}
