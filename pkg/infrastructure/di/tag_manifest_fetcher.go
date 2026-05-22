package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/tagmanifestfetcher"
)

func (c *Container) getTagManifestFetcher() *tagmanifestfetcher.FileSystem {
	if c.tagManifestFetcher == nil {
		l, err := tagmanifestfetcher.NewFileSystem(
			c.GetLogger(),
			c.getImageFinder(),
			c.config.FS.Root,
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create tag manifest fetcher",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.tagManifestFetcher = l
	}

	return c.tagManifestFetcher
}
