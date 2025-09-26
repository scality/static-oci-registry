package logger

import (
	"os"

	"github.com/rs/zerolog"
)

func NewZeroLog(logLevel string) *zerolog.Logger {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		// don't fail here, but default to debug
		level = zerolog.DebugLevel
	}

	l := zerolog.
		New(os.Stdout).
		Level(level).
		With().
		Timestamp().
		Logger()

	hostname, err := os.Hostname()
	if err == nil {
		// don't fail here, just ignore hostname if not found
		l = l.With().Str("host_name", hostname).Logger()
	}

	return &l
}
