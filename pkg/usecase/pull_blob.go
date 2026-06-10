package usecase

import (
	"context"
	"io"
	"log/slog"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

type PullBlob struct {
	logger     *slog.Logger
	blobPuller service.BlobPuller
}

func NewPullBlob(
	logger *slog.Logger,
	blobPuller service.BlobPuller,
) *PullBlob {
	return &PullBlob{
		logger:     logger.With(slog.String("use_case", "pull_blob")),
		blobPuller: blobPuller,
	}
}

func (uc *PullBlob) Execute(
	ctx context.Context,
	imageName domain.ImageName,
	digest domain.Digest,
) (io.ReadSeekCloser, error) {
	l := uc.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Pulling blob")

	readseekcloser, err := uc.blobPuller.PullBlob(ctx, imageName, digest)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to pull blob"))
	}

	return readseekcloser, nil
}
