package layoutwalker

import (
	"context"
	"iter"
	"log/slog"
	"os"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
	"github.com/scality/static-oci-registry/pkg/service"
)

// FileSystem yields one ocilayout.Layout per (solution, version) candidate that
// contains the image, in the priority order returned by the ImageFinder.
type FileSystem struct {
	logger      *slog.Logger
	imageFinder service.ImageFinder
	root        *os.Root
}

var _ service.LayoutWalker = (*FileSystem)(nil)

func NewFileSystem(
	logger *slog.Logger,
	imageFinder service.ImageFinder,
	root *os.Root,
) (*FileSystem, error) {
	return &FileSystem{
		logger:      logger.With(slog.String("layout_walker", "filesystem")),
		imageFinder: imageFinder,
		root:        root,
	}, nil
}

func (fs *FileSystem) WalkLayouts(
	ctx context.Context, imageName domain.ImageName,
) iter.Seq2[service.Layout, error] {
	return func(yield func(service.Layout, error) bool) {
		candidates, err := fs.imageFinder.FindImage(ctx, imageName)
		if err != nil {
			yield(nil, errors.Wrap(err,
				errors.WithDetail("failed to find image in filesystem registry")))

			return
		}

		for _, sv := range candidates {
			if !yield(ocilayout.NewLayout(fs.logger, fs.root, sv, imageName), nil) {
				return
			}
		}
	}
}
