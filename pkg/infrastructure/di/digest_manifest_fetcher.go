package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
)

func (c *Container) getDigestManifestFetcher() *digestmanifestfetcher.FileSystem {
	if c.digestManifestFetcher == nil {
		l, err := digestmanifestfetcher.NewFileSystem(
			c.GetLogger(),
			c.getImageFinder(),
			c.config.FS.Root,
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create digest manifest fetcher",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.digestManifestFetcher = l
	}

	return c.digestManifestFetcher
}
