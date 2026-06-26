package ocilayout

import (
	"bytes"
	"context"
	"io"
	"iter"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

// nopCloserReadSeeker adapts a *bytes.Reader to io.ReadSeekCloser for the mock.
type nopCloserReadSeeker struct{ *bytes.Reader }

func (nopCloserReadSeeker) Close() error { return nil }

// MockLayout is a configurable in-memory service.Layout for unit tests.
// A nil entry in ByTag/ByDigest/Blobs (i.e. absent key) yields the
// (nil, nil) "not present here" contract.
type MockLayout struct {
	TagList    []domain.Tag
	ByTag      map[domain.Tag]*domain.FetchManifestOutput
	ByDigest   map[domain.Digest]*domain.FetchManifestOutput
	Blobs      map[domain.Digest][]byte
	TagsErr    error
	ResolveErr error
	DigestErr  error
	BlobErr    error
	Path       string
}

// NewMockLayout returns an initialised MockLayout ready for use in tests.
func NewMockLayout() *MockLayout {
	return &MockLayout{
		ByTag:    make(map[domain.Tag]*domain.FetchManifestOutput),
		ByDigest: make(map[domain.Digest]*domain.FetchManifestOutput),
		Blobs:    make(map[domain.Digest][]byte),
	}
}

// Tags implements service.Layout.
func (m *MockLayout) Tags(context.Context) ([]domain.Tag, error) {
	if m.TagsErr != nil {
		return nil, m.TagsErr
	}

	return m.TagList, nil
}

// Location implements service.Layout.
func (m *MockLayout) Location() string {
	return m.Path
}

// ResolveTag implements service.Layout.
func (m *MockLayout) ResolveTag(
	_ context.Context, tag domain.Tag,
) (*domain.FetchManifestOutput, error) {
	if m.ResolveErr != nil {
		return nil, m.ResolveErr
	}

	return m.ByTag[tag], nil
}

// ReadManifestByDigest implements service.Layout.
func (m *MockLayout) ReadManifestByDigest(
	_ context.Context, digest domain.Digest,
) (*domain.FetchManifestOutput, error) {
	if m.DigestErr != nil {
		return nil, m.DigestErr
	}

	return m.ByDigest[digest], nil
}

// OpenBlob implements service.Layout.
func (m *MockLayout) OpenBlob(_ context.Context, digest domain.Digest) (io.ReadSeekCloser, error) {
	if m.BlobErr != nil {
		return nil, m.BlobErr
	}

	raw, ok := m.Blobs[digest]
	if !ok {
		return nil, nil //nolint:nilnil // (nil,nil) means "not present here" per service.Layout contract
	}

	return nopCloserReadSeeker{bytes.NewReader(raw)}, nil
}

// MockWalker is a configurable service.LayoutWalker for unit tests. It yields
// the configured layouts in order; set WalkErr to simulate an imagefinder-level
// failure (yielded once, then iteration stops).
type MockWalker struct {
	Layouts []service.Layout
	WalkErr error
}

// NewMockWalker returns an initialised MockWalker ready for use in tests.
func NewMockWalker() *MockWalker { return &MockWalker{} }

// Add appends a layout to the walker's sequence.
func (m *MockWalker) Add(l service.Layout) { m.Layouts = append(m.Layouts, l) }

// WalkLayouts implements service.LayoutWalker.
func (m *MockWalker) WalkLayouts(
	_ context.Context, _ domain.ImageName,
) iter.Seq2[service.Layout, error] {
	return func(yield func(service.Layout, error) bool) {
		if m.WalkErr != nil {
			yield(nil, m.WalkErr)

			return
		}

		for _, l := range m.Layouts {
			if !yield(l, nil) {
				return
			}
		}
	}
}
