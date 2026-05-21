package service

import (
	"context"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/errors"
)

type ImageFinder interface {
	FindImage(
		ctx context.Context,
		imageName domain.ImageName,
	) ([]domain.SolutionVersion, *errors.Error)
}
