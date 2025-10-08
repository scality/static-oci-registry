package usecase

import (
	"slices"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/errors"
	"github.com/scality/static-oci-registry/pkg/service"

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

func (uc *ListTags) Execute(input domain.ListTagsInput) (*domain.ListTagsOutput, *errors.Error) {
	l := uc.logger.With().Str("image_name", string(input.Name)).Logger()
	l.Info().Msg("Listing tags for image")

	listTagsOutput, err := uc.tagLister.ListTags(input.Name)
	if err != nil {
		return nil, err.Wrap("failed to list tags")
	}

	if input.Last != nil {
		if !slices.Contains(listTagsOutput.Tags, *input.Last) {
			return nil, errors.FromCode(domain.ErrTagNotFound, ocierrors.Unsupported).
				WithOCIMessage(domain.ErrTagNotFound.Error()).
				WithOCIDetail("last", string(*input.Last))
		}

		// return tags after lastTag without lastTag
		i := slices.Index(listTagsOutput.Tags, *input.Last)
		listTagsOutput.Tags = listTagsOutput.Tags[i+1:]
	}

	if input.N != nil {
		if *input.N < len(listTagsOutput.Tags) {
			listTagsOutput.Tags = listTagsOutput.Tags[:*input.N]
		}
	}

	return listTagsOutput, nil
}
