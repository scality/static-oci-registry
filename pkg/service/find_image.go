package service

import (
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/errors"
)

type ImageFinder interface {
	FindImage(imageName domain.ImageName) ([]string, *errors.Error)
}
