package tagmanifestfetcher

import (
	"context"
	"log/slog"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger       *slog.Logger
	layoutWalker service.LayoutWalker
}

var _ service.TagManifestFetcher = (*FileSystem)(nil)

func NewFileSystem(logger *slog.Logger, layoutWalker service.LayoutWalker) (*FileSystem, error) {
	return &FileSystem{
		logger:       logger.With(slog.String("tag_manifest_fetcher", "filesystem")),
		layoutWalker: layoutWalker,
	}, nil
}

func (fs *FileSystem) FetchManifest(
	ctx context.Context, imageName domain.ImageName, tag domain.Tag,
) (*domain.FetchManifestOutput, error) {
	l := fs.logger.With(slog.String("image_name", string(imageName)), slog.String("tag", string(tag)))
	l.InfoContext(ctx, "Finding manifest in filesystem registry")

	for layout, err := range fs.layoutWalker.WalkLayouts(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(err,
				errors.WithDetail("failure while walking layouts in filesystem registry"))
		}

		out, err := layout.ResolveTag(ctx, tag)
		if err != nil {
			l.WarnContext(ctx, "failed to resolve tag in a layout, skipping",
				slog.String("layout", layout.Location()), slog.Any("error", err))

			continue
		}

		if out == nil {
			continue
		}

		out.SolutionVersion = layout.SolutionVersion()

		return out, nil
	}

	return nil, errors.Wrap(
		domain.ErrManifestNotFound,
		errors.WithDetail("manifest not found in filesystem registry"),
		ocierrors.BuildOCIProperties(
			ocierrors.ManifestUnknown,
			domain.ErrManifestNotFound.Error(),
			map[string]string{"tag": tag.String()},
		),
	)
}
