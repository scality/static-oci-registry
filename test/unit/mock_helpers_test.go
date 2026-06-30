package unit

import (
	"bytes"
	"io"
	"iter"

	"github.com/scality/static-oci-registry/pkg/service"
)

// layoutSeq builds an iter.Seq2 yielding the given layouts in order with no
// error -- the value returned by a mocked LayoutWalker.WalkLayouts.
func layoutSeq(layouts ...service.Layout) iter.Seq2[service.Layout, error] {
	return func(yield func(service.Layout, error) bool) {
		for _, l := range layouts {
			if !yield(l, nil) {
				return
			}
		}
	}
}

// errSeq builds an iter.Seq2 that yields a single walker-level error (the
// imagefinder-failure path) and stops.
func errSeq(err error) iter.Seq2[service.Layout, error] {
	return func(yield func(service.Layout, error) bool) {
		yield(nil, err)
	}
}

// nopCloserReadSeeker adapts a *bytes.Reader to io.ReadSeekCloser, for mocking
// Layout.OpenBlob return values.
type nopCloserReadSeeker struct{ *bytes.Reader }

func (nopCloserReadSeeker) Close() error { return nil }

func blobReader(b []byte) io.ReadSeekCloser {
	return nopCloserReadSeeker{bytes.NewReader(b)}
}
