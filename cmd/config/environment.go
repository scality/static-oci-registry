package config

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sethvargo/go-envconfig"
)

// ApplicationVersion is the version of the application.
// It is set at build time using ldflags.
//
//nolint:gochecknoglobals // This is a constant.
var ApplicationVersion = "dev"

const ApplicationName = "static-oci-registry"

type (
	Environment struct {
		LogLevel string `env:"LOG_LEVEL, default=info"`
		HTTP     HTTP   `env:",prefix=HTTP_"`
	}
	HTTP struct {
		Addr string `env:"ADDR, default=:8080"`
	}
)

func NewEnvironment(ctx context.Context) (*Environment, error) {
	cfg := &Environment{}

	err := cfg.Load(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading environment variables")
	}

	return cfg, nil
}

func (cfg *Environment) Load(ctx context.Context) error {
	err := envconfig.Process(ctx, cfg)
	if err != nil {
		return errors.Wrap(err, "failed loading config")
	}

	return nil
}
