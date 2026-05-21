package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
)

type ImageFinder interface {
	FindImage(
		ctx context.Context,
		imageName domain.ImageName,
	) ([]domain.SolutionVersion, error)
}
