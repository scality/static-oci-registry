package usecase

import (
	"context"
	"log/slog"
	"slices"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

type ListTags struct {
	logger    *slog.Logger
	tagLister service.TagLister
}

func NewListTags(
	logger *slog.Logger,
	tagLister service.TagLister,
) *ListTags {
	return &ListTags{
		logger:    logger.With(slog.String("use_case", "list_tags")),
		tagLister: tagLister,
	}
}

func (uc *ListTags) Execute(ctx context.Context, input domain.ListTagsInput) (
	*domain.ListTagsOutput, error,
) {
	l := uc.logger.With(slog.String("image_name", string(input.Name)))
	l.InfoContext(ctx, "Listing tags for image")

	listTagsOutput, err := uc.tagLister.ListTags(ctx, input.Name)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to list tags"))
	}

	if input.Last != nil {
		if !slices.Contains(listTagsOutput.Tags, *input.Last) {
			return nil, errors.Wrap(
				domain.ErrTagNotFound,
				ocierrors.BuildOCIProperties(
					ocierrors.Unsupported,
					domain.ErrTagNotFound.Error(),
					map[string]string{"last": string(*input.Last)},
				),
			)
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
