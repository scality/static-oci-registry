package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
)

func (c *Container) getTagWalker() *tagwalker.FileSystem {
	if c.tagWalker == nil {
		l, err := tagwalker.NewFileSystem(
			c.GetLogger(),
			c.getImageFinder(),
			c.getFSRoot(),
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create tag walker",
				slog.Any("error", err),
			)
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.tagWalker = l
	}

	return c.tagWalker
}
