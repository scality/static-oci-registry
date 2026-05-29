package usecase

import (
	"context"
	"log/slog"
	"slices"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
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
		// Mirror the reference distribution registry
		// (distribution/distribution, registry/handlers/tags.go): if `last`
		// is not in the (sorted) tag list, return the full list unchanged
		// rather than an error. The OCI v1.0.1 spec does not define an
		// error code for an unknown `last` value, so the previous
		// UNSUPPORTED response was a misuse.
		if i := slices.Index(listTagsOutput.Tags, *input.Last); i >= 0 {
			listTagsOutput.Tags = listTagsOutput.Tags[i+1:]
		}
	}

	if input.N != nil {
		if *input.N < len(listTagsOutput.Tags) {
			listTagsOutput.Tags = listTagsOutput.Tags[:*input.N]
		}
	}

	return listTagsOutput, nil
}
