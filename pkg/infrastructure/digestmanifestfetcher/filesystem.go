package digestmanifestfetcher

import (
	"context"
	"log/slog"
	"os"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger      *slog.Logger
	imageFinder service.ImageFinder
	fsRoot      string
}

func NewFileSystem(
	l *slog.Logger,
	i service.ImageFinder,
	r string,
) (*FileSystem, error) {
	info, err := os.Stat(r)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("unable to access FS_ROOT"))
	}

	if !info.IsDir() {
		return nil, errors.New("passed FS_ROOT is not a directory")
	}

	return &FileSystem{
		logger:      l.With(slog.String("digest_manifest_fetcher", "filesystem")),
		imageFinder: i,
		fsRoot:      r,
	}, nil
}

func (fs *FileSystem) FetchManifest(ctx context.Context, imageName domain.ImageName, digest domain.Digest) (
	*domain.FetchManifestOutput, error,
) {
	// TODO: implement this function
	return nil, nil
}
