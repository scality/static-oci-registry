package di

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagmanifestfetcher"
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

	listTagsHandler      *handler.ListTags
	fetchManifestHandler *handler.FetchManifest

	listTagsUseCase                *usecase.ListTags
	fetchManifestFromTagUseCase    *usecase.FetchManifestFromTag
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest

	tagLister   *taglister.FileSystem
	imageFinder *imagefinder.FileSystem

	tagManifestFetcher    *tagmanifestfetcher.FileSystem
	digestManifestFetcher *digestmanifestfetcher.FileSystem
}

func NewContainer(ctx context.Context, cfg *config.Environment) *Container {
	return &Container{
		ctx:    ctx,
		config: cfg,
	}
}
