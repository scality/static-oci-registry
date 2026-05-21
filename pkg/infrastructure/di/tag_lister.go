package di

import (
	"log/slog"
	"os"

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
			c.GetLogger().ErrorContext(c.ctx, "failed to create tag lister",
				slog.Any("error_message", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.tagLister = l
	}

	return c.tagLister
}
