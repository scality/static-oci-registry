package service

import (
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/errors"
)

type TagLister interface {
	ListTags(imageName domain.ImageName) (*domain.ListTagsOutput, *errors.Error)
}
