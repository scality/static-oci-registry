package service

import (
	"github.com/scality/static-oci-registry/pkg/domain"
)

type ImageFinder interface {
	FindImage(imageName domain.ImageName) ([]domain.SolutionVersion, error)
}
