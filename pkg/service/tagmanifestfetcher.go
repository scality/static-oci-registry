package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type TagManifestFetcher interface {
	FetchManifest(
		ctx context.Context,
		imageName domain.ImageName,
		tag domain.Tag,
	) (*domain.Manifest, error)
}
