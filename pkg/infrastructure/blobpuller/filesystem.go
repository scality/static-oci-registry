package blobpuller

import (
	"context"
	"io"
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

var _ service.BlobPuller = (*FileSystem)(nil)

func NewFileSystem(logger *slog.Logger, layoutWalker service.LayoutWalker) (*FileSystem, error) {
	return &FileSystem{
		logger:       logger.With(slog.String("blob_puller", "filesystem")),
		layoutWalker: layoutWalker,
	}, nil
}

func (fs *FileSystem) PullBlob(
	ctx context.Context, imageName domain.ImageName, digest domain.Digest,
) (io.ReadSeekCloser, error) {
	l := fs.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Pulling blob from filesystem registry")

	for layout, err := range fs.layoutWalker.WalkLayouts(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(err,
				errors.WithDetail("failure while walking layouts in filesystem registry"))
		}

		rc, err := layout.OpenBlob(ctx, digest)
		if err != nil {
			l.WarnContext(ctx, "failed to open blob in a layout, skipping",
				slog.String("layout", layout.Location()), slog.Any("error", err))

			continue
		}

		if rc == nil {
			continue
		}

		return rc, nil
	}

	return nil, errors.Wrap(
		domain.ErrBlobNotFound,
		errors.WithDetail("blob not found in filesystem registry"),
		ocierrors.BuildOCIProperties(
			ocierrors.BlobUnknown,
			domain.ErrBlobNotFound.Error(),
			map[string]string{"digest": string(digest)},
		),
	)
}
