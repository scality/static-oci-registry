package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type TagLister interface {
	ListTags(
		ctx context.Context,
		imageName domain.ImageName,
	) (*domain.ListTagsOutput, error)
}
