// FetchManifestFromDigest and FetchManifestFromTag are very similar
// but combining them would couple dispatch logic (tag vs. digest) to the usecase
//
// nolint:dupl // see above
package usecase

import (
	"context"
	"log/slog"

	"github.com/scality/go-errors"
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
	l := uc.logger.With(slog.String("image_name", string(imageName)), slog.String("tag", string(tag)))
	l.InfoContext(ctx, "Fetching manifest for tag")

	fetchManifestOutput, err := uc.tagManifestFetcher.FetchManifest(ctx, imageName, tag)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to fetch manifest from tag"))
	}

	return fetchManifestOutput, nil
}
