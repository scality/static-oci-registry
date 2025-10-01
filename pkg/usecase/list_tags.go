package usecase

import (
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type ListTags struct {
	logger    *zerolog.Logger
	tagLister service.TagLister
}

func NewListTags(
	logger *zerolog.Logger,
	tagLister service.TagLister,
) *ListTags {
	l := logger.With().Str("use_case", "list_tags").Logger()

	return &ListTags{
		logger:    &l,
		tagLister: tagLister,
	}
}

func (uc *ListTags) Execute(imageName domain.ImageName) (*domain.ListTagsOutput, error) {
	l := uc.logger.With().Str("image_name", string(imageName)).Logger()
	l.Info().Msg("Listing tags for image")

	listTagsOutput, err := uc.tagLister.ListTags(imageName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tags")
	}

	return listTagsOutput, nil
}
