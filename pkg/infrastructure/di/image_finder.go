package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
)

func (c *Container) getImageFinder() *imagefinder.FileSystem {
	if c.imageFinder == nil {
		i, err := imagefinder.NewFileSystem(
			c.GetLogger(),
			c.getFSRoot(),
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create image finder",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.imageFinder = i
	}

	return c.imageFinder
}
