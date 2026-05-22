package usecase

import (
	"context"
	"log/slog"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FetchManifestFromDigest struct {
	logger                *slog.Logger
	digestManifestFetcher service.DigestManifestFetcher
}

func NewFetchManifestFromDigest(
	logger *slog.Logger,
	digestManifestFetcher service.DigestManifestFetcher,
) *FetchManifestFromDigest {
	return &FetchManifestFromDigest{
		logger:                logger.With(slog.String("use_case", "fetch_manifest_from_digest")),
		digestManifestFetcher: digestManifestFetcher,
	}
}

func (uc *FetchManifestFromDigest) Execute(
	ctx context.Context,
	imageName domain.ImageName,
	digest domain.Digest,
) (*domain.Manifest, error) {
	// TODO: implement this use case
	return nil, nil
}
