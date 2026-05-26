package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type DigestManifestFetcher interface {
	FetchManifest(
		ctx context.Context,
		imageName domain.ImageName,
		digest domain.Digest,
	) (*domain.FetchManifestOutput, error)
}
