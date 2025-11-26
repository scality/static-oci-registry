package di

import (
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
)

func (c *Container) getImageFinder() *imagefinder.FileSystem {
	if c.imageFinder == nil {
		i, err := imagefinder.NewFileSystem(
			c.GetLogger(),
			c.config.FS.Root,
		)
		if err != nil {
			c.GetLogger().Fatal().Err(err).Msg("failed to create image finder")
		}

		c.imageFinder = i
	}

	return c.imageFinder
}
