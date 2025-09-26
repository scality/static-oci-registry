package di

import (
	"static-oci-registry/cmd/config"
	"static-oci-registry/pkg/infrastructure/logger"

	"github.com/rs/zerolog"
)

func (c *Container) GetLogger() *zerolog.Logger {
	if c.logger == nil {
		l := logger.NewZeroLog(c.config.LogLevel).
			With().
			Str("application_name", config.ApplicationName).
			Str("application_version", config.ApplicationVersion).
			Logger()

		c.logger = &l
	}

	return c.logger
}
