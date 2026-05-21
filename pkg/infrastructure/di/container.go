package di

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/pkg/presentation/http/handler"
	"github.com/scality/static-oci-registry/pkg/usecase"
)

type Container struct {
	// ctx is the base context for the application.
	ctx context.Context //nolint:containedctx

	config *config.Environment

	logger *slog.Logger

	TLSConfig  *tls.Config
	router     http.Handler
	httpServer *http.Server

	listTagsHandler *handler.ListTags

	listTagsUseCase *usecase.ListTags

	tagLister   *taglister.FileSystem
	imageFinder *imagefinder.FileSystem
}

func NewContainer(ctx context.Context, cfg *config.Environment) *Container {
	return &Container{
		ctx:    ctx,
		config: cfg,
	}
}
