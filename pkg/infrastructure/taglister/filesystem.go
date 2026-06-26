package taglister

import (
	"context"
	"log/slog"
	"slices"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

// FileSystem lists tags by walking all OCI Image Layout candidates for an image.
type FileSystem struct {
	logger       *slog.Logger
	layoutWalker service.LayoutWalker
}

var _ service.TagLister = (*FileSystem)(nil)

// NewFileSystem creates a FileSystem tag lister backed by the given LayoutWalker.
func NewFileSystem(logger *slog.Logger, layoutWalker service.LayoutWalker) (*FileSystem, error) {
	return &FileSystem{
		logger:       logger.With(slog.String("tag_lister", "filesystem")),
		layoutWalker: layoutWalker,
	}, nil
}

// ListTags returns a sorted, deduplicated list of tags for the given image.
func (fs *FileSystem) ListTags(ctx context.Context, imageName domain.ImageName) (
	*domain.ListTagsOutput, error,
) {
	l := fs.logger.With(slog.String("image_name", string(imageName)))
	l.InfoContext(ctx, "Finding tags in filesystem registry")

	var allTags []domain.Tag

	for layout, err := range fs.layoutWalker.WalkLayouts(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(err,
				errors.WithDetail("failure while walking layouts in filesystem registry"))
		}

		tags, err := layout.Tags(ctx)
		if err != nil {
			l.WarnContext(ctx, "failed to read tags from a layout, skipping",
				slog.String("layout", layout.Location()), slog.Any("error", err))

			continue
		}

		for _, t := range tags {
			if slices.Contains(allTags, t) {
				l.With(slog.String("tag", t.String())).WarnContext(ctx, "duplicate tag found, skipping")

				continue
			}

			allTags = append(allTags, t)
		}
	}

	slices.SortFunc(allTags, domain.CompareTags)

	return &domain.ListTagsOutput{Name: imageName, Tags: allTags}, nil
}
