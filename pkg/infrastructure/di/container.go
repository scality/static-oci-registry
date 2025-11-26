package di

import (
	"context"
	"net/http"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/pkg/presentation/http/handler"
	"github.com/scality/static-oci-registry/pkg/usecase"

	"github.com/rs/zerolog"
)

type Container struct {
	// baseCtx is the base context for the application.
	baseCtx context.Context //nolint:containedctx

	config *config.Environment

	logger *zerolog.Logger

	router http.Handler

	httpServer *http.Server

	listTagsHandler *handler.ListTags

	listTagsUseCase *usecase.ListTags

	tagLister   *taglister.FileSystem
	imageFinder *imagefinder.FileSystem
}

func NewContainer(ctx context.Context, cfg *config.Environment) *Container {
	return &Container{
		baseCtx: ctx,
		config:  cfg,
	}
}
