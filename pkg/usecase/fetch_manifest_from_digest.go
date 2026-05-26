package usecase

import (
	"context"
	"log/slog"

	"github.com/scality/go-errors"
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
) (*domain.FetchManifestOutput, error) {
	l := uc.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Fetching manifest for digest")

	fetchManifestOutput, err := uc.digestManifestFetcher.FetchManifest(ctx, imageName, digest)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to fetch manifest from digest"))
	}

	return fetchManifestOutput, nil
}
