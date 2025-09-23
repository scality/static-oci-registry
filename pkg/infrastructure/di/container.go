package di

import (
	"context"
	"net/http"
	"platform-static-registry/cmd/config"

	"github.com/rs/zerolog"
)

type Container struct {
	// baseCtx is the base context for the application.
	baseCtx context.Context //nolint:containedctx

	config *config.Environment

	logger *zerolog.Logger

	httpServer *http.Server
}

func NewContainer(ctx context.Context, cfg *config.Environment) *Container {
	return &Container{
		baseCtx: ctx,
		config:  cfg,
	}
}
