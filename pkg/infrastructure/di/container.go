package di

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/blobpuller"
	"github.com/scality/static-oci-registry/pkg/infrastructure/certwatcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/layoutwalker"
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

	TLSConfig       *tls.Config
	router          http.Handler
	httpCertWatcher *certwatcher.CertWatcher
	httpServer      *http.Server

	metricsServer      *http.Server
	metricsHandler     http.Handler
	metricsTLSConfig   *tls.Config
	metricsCertWatcher *certwatcher.CertWatcher
	metricsRegistry *prometheus.Registry

	listTagsHandler      *handler.ListTags
	fetchManifestHandler *handler.FetchManifest
	pullBlobHandler      *handler.PullBlob
	unsupportedHandler   *handler.UnsupportedEndpoint

	listTagsUseCase                *usecase.ListTags
	fetchManifestFromTagUseCase    *usecase.FetchManifestFromTag
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest
	pullBlobUseCase                *usecase.PullBlob

	imageFinder           *imagefinder.FileSystem
	layoutWalker          *layoutwalker.FileSystem
	tagLister             *taglister.FileSystem
	tagManifestFetcher    *tagmanifestfetcher.FileSystem
	digestManifestFetcher *digestmanifestfetcher.FileSystem
	blobPuller            *blobpuller.FileSystem

	fsRoot *os.Root
}

func NewContainer(ctx context.Context, cfg *config.Environment) *Container {
	return &Container{
		ctx:    ctx,
		config: cfg,
	}
}
