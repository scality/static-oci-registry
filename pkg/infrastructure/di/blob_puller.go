package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/blobpuller"
)

func (c *Container) getBlobPuller() *blobpuller.FileSystem {
	if c.blobPuller == nil {
		p, err := blobpuller.NewFileSystem(
			c.GetLogger(),
			c.getTagWalker(),
			c.config.FS.Root,
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create blob puller",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.blobPuller = p
	}

	return c.blobPuller
}
