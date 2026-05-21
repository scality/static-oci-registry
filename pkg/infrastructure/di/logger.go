package di

import (
	"log/slog"
	"os"

	"github.com/scality/static-oci-registry/cmd/config"
)

func (c *Container) GetLogger() *slog.Logger {
	if c.logger == nil {
		var level slog.Level
		if err := level.UnmarshalText([]byte(c.config.LogLevel)); err != nil {
			// don't fail here, but default to debug
			level = slog.LevelDebug
		}

		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

		logger := slog.New(handler).With(
			slog.String("application_name", config.ApplicationName),
			slog.String("application_version", config.ApplicationVersion),
		)

		if hostname, err := os.Hostname(); err == nil {
			// don't fail here, just ignore hostname if not found
			logger = logger.With(slog.String("host_name", hostname))
		}

		c.logger = logger
	}

	return c.logger
}
