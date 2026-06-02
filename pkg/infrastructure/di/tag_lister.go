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
			c.getTagWalker(),
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create tag lister",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.tagLister = l
	}

	return c.tagLister
}
