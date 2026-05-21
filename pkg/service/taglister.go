package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/errors"
)

type TagLister interface {
	ListTags(
		ctx context.Context,
		imageName domain.ImageName,
	) (*domain.ListTagsOutput, *errors.Error)
}
