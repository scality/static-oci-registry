package usecase

import (
	"context"
	"log/slog"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FetchManifestFromTag struct {
	logger             *slog.Logger
	tagManifestFetcher service.TagManifestFetcher
}

func NewFetchManifestFromTag(
	logger *slog.Logger,
	tagManifestFetcher service.TagManifestFetcher,
) *FetchManifestFromTag {
	return &FetchManifestFromTag{
		logger:             logger.With(slog.String("use_case", "fetch_manifest_from_tag")),
		tagManifestFetcher: tagManifestFetcher,
	}
}

func (uc *FetchManifestFromTag) Execute(
	ctx context.Context,
	imageName domain.ImageName,
	tag domain.Tag,
) (*domain.FetchManifestOutput, error) {
	// TODO: implement this use case
	return nil, nil
}
