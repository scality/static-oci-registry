package di

import (
	"log/slog"
	"os"
)

func (c *Container) getFSRoot() *os.Root {
	if c.fsRoot == nil {
		root, err := os.OpenRoot(c.config.FS.Root)
		if err != nil {
			c.GetLogger().ErrorContext(c.ctx, "failed to open FS root",
				slog.Any("error", err),
				slog.String("rootPath", c.config.FS.Root),
			)

			os.Exit(1) //nolint:revive // Fatal-equivalent for DI initialization failure
		}

		c.fsRoot = root
	}

	return c.fsRoot
}
