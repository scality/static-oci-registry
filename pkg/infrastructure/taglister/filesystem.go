// nolint:cyclop // this is complex infrastructure logic by nature
package taglister

import (
	"context"
	"log/slog"
	"slices"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger    *slog.Logger
	tagWalker service.TagWalker
}

func NewFileSystem(
	logger *slog.Logger,
	tagWalker service.TagWalker,
) (*FileSystem, error) {
	return &FileSystem{
		logger:    logger.With(slog.String("tag_lister", "filesystem")),
		tagWalker: tagWalker,
	}, nil
}

func (fs *FileSystem) ListTags(ctx context.Context, imageName domain.ImageName) (
	*domain.ListTagsOutput, error,
) {
	l := fs.logger.With(slog.String("image_name", string(imageName)))
	l.InfoContext(ctx, "Finding tags in filesystem registry")

	var allTags []domain.Tag

	for entry, err := range fs.tagWalker.WalkTags(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("failure while walking tags in filesystem registry"),
			)
		}

		if slices.Contains(allTags, entry.Tag) {
			l.With(slog.String("tag", entry.Tag.String())).WarnContext(ctx, "duplicate tag found, skipping")

			continue
		}

		allTags = append(allTags, entry.Tag)
	}

	// sort Tags
	slices.SortFunc(allTags, domain.CompareTags)

	return &domain.ListTagsOutput{Name: imageName, Tags: allTags}, nil
}
