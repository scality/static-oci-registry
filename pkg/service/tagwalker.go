package service

import (
	"context"
	"iter"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type TagWalker interface {
	WalkTags(
		ctx context.Context,
		imageName domain.ImageName,
	) iter.Seq2[domain.TagEntry, error]

	ReadManifestBytes(entry domain.TagEntry) ([]byte, error)
}
