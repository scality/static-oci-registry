package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/pkg/infrastructure/layoutwalker"
)

func (c *Container) getLayoutWalker() *layoutwalker.FileSystem {
	if c.layoutWalker == nil {
		w, err := layoutwalker.NewFileSystem(
			c.GetLogger(),
			c.getImageFinder(),
			c.getFSRoot(),
		)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to create layout walker",
				slog.Any("error", err))
			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.layoutWalker = w
	}

	return c.layoutWalker
}
