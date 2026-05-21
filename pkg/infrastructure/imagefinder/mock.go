package imagefinder

import (
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/errors"
)

type Mock struct {
	data  map[domain.ImageName][]domain.SolutionVersion
	error *errors.Error
}

func NewMock() *Mock {
	return &Mock{
		data:  make(map[domain.ImageName][]domain.SolutionVersion),
		error: nil,
	}
}

func (m *Mock) FindImage(imageName domain.ImageName) ([]domain.SolutionVersion, *errors.Error) {
	if m.error != nil {
		return nil, m.error
	}

	if contents, ok := m.data[imageName]; ok {
		return contents, nil
	}

	return nil, errors.FromCode(domain.ErrImageNotFound, ocierrors.Unsupported).
		Wrap("image not found in filesystem registry").
		WithOCIMessage(domain.ErrImageNotFound.Error()).
		WithOCIDetail("image_name", string(imageName))
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

func (m *Mock) SetError(err *errors.Error) {
	m.error = err
}
