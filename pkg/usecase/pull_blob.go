// PullBlob is structurally similar to FetchManifestFromDigest but semantically
// distinct: it streams blob bytes rather than returning a decoded manifest.
//
//nolint:dupl // see above
package usecase

import (
	"context"
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
) (*domain.PullBlobOutput, error) {
	l := uc.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Pulling blob")

	out, err := uc.blobPuller.PullBlob(ctx, imageName, digest)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to pull blob"))
	}

	return out, nil
}
