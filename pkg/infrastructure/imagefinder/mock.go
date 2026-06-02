package imagefinder

import (
	"context"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

type Mock struct {
	data  map[domain.ImageName][]domain.SolutionVersion
	error error
}

func NewMock() *Mock {
	return &Mock{
		data:  make(map[domain.ImageName][]domain.SolutionVersion),
		error: nil,
	}
}

func (m *Mock) FindImage(_ context.Context, imageName domain.ImageName) (
	[]domain.SolutionVersion, error,
) {
	if m.error != nil {
		return nil, m.error
	}

	if contents, ok := m.data[imageName]; ok {
		return contents, nil
	}

	return nil, errors.Wrap(
		domain.ErrImageNotFound,
		errors.WithDetail("image not found in filesystem registry"),
		ocierrors.BuildOCIProperties(
			ocierrors.NameUnknown,
			domain.ErrImageNotFound.Error(),
			map[string]string{"image_name": string(imageName)},
		),
	)
}

func (m *Mock) RemoveErrors() {
	m.error = nil
}

func (m *Mock) AddImage(solution, version string, image domain.ImageName) {
	sv := domain.SolutionVersion{
		Solution: solution,
		Version:  version,
	}

	if contents, ok := m.data[image]; ok {
		m.data[image] = append(contents, sv)
	} else {
		m.data[image] = []domain.SolutionVersion{sv}
	}
}

func (m *Mock) SetError(err error) {
	m.error = err
}
