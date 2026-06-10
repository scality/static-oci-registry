package service

import (
	"context"
	"io"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type BlobPuller interface {
	PullBlob(
		ctx context.Context,
		imageName domain.ImageName,
		digest domain.Digest,
	) (io.ReadSeekCloser, error)
}
