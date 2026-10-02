package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type BlobPuller interface {
	PullBlob(
		ctx context.Context,
		imageName domain.ImageName,
		digest domain.Digest,
	) (*domain.PullBlobOutput, error)
}
