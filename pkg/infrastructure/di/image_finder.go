package di

import (
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/service"
)

func (c *Container) getImageFinder() service.ImageFinder {
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
