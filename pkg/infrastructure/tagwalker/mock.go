package tagwalker

import (
	"context"
	"iter"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

type Mock struct {
	entries   []domain.TagEntry
	manifests map[string][]byte
	walkErr   error
	readErr   error
}

func NewMock() *Mock {
	return &Mock{
		entries:   nil,
		manifests: make(map[string][]byte),
		walkErr:   nil,
		readErr:   nil,
	}
}

func (m *Mock) AddEntry(entry domain.TagEntry, manifestBytes []byte) {
	m.entries = append(m.entries, entry)
	m.manifests[mockKey(entry)] = manifestBytes
}

func (m *Mock) SetWalkError(err error) {
	m.walkErr = err
}

func (m *Mock) SetReadError(err error) {
	m.readErr = err
}

func (m *Mock) Reset() {
	m.entries = nil
	m.manifests = make(map[string][]byte)
	m.walkErr = nil
	m.readErr = nil
}

func (m *Mock) WalkTags(_ context.Context, _ domain.ImageName) iter.Seq2[domain.TagEntry, error] {
	return func(yield func(domain.TagEntry, error) bool) {
		if m.walkErr != nil {
			yield(domain.TagEntry{}, m.walkErr)

			return
		}

		for _, entry := range m.entries {
			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (m *Mock) ReadManifestBytes(entry domain.TagEntry) ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}

	bytes, ok := m.manifests[mockKey(entry)]
	if !ok {
		return nil, errors.Wrap(
			domain.ErrManifestNotFound,
			errors.WithDetail("manifest not found in tagwalker mock"),
			ocierrors.BuildOCIProperties(
				ocierrors.ManifestUnknown,
				domain.ErrManifestNotFound.Error(),
				map[string]string{
					"image_name": entry.Name.String(),
					"tag":        entry.Tag.String(),
				},
			),
		)
	}

	return bytes, nil
}

func mockKey(entry domain.TagEntry) string {
	return entry.Solution + "/" + entry.Version + "/" + entry.Name.String() + "/" + entry.Tag.String()
}
