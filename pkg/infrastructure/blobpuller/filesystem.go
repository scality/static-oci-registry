package blobpuller

import (
	"context"
	"io"
	"log/slog"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger    *slog.Logger
	tagWalker service.TagWalker
}

func NewFileSystem(
	l *slog.Logger,
	t service.TagWalker,
) (*FileSystem, error) {
	return &FileSystem{
		logger:    l.With(slog.String("blob_puller", "filesystem")),
		tagWalker: t,
	}, nil
}

func (fs *FileSystem) PullBlob(ctx context.Context, imageName domain.ImageName, digest domain.Digest) (io.ReadSeekCloser, error) {
	return nil, nil
}
