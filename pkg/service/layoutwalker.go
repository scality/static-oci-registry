package service

import (
	"context"
	"io"
	"iter"

	"github.com/scality/static-oci-registry/pkg/domain"
)

// Layout is a request-scoped reader over a single image's OCI Image Layout
// (one <solution>/<version>/<image> directory). Implementations read on demand
// and MUST NOT cache across requests. A nil result with a nil error means
// "not present in this layout" so callers can try the next candidate.
type Layout interface {
	// Location returns a human-readable identifier of this layout (its
	// <solution>/<version>/<image> path) for diagnostic logging.
	Location() string

	// Tags returns the tags advertised by this layout's index.json
	// (org.opencontainers.image.ref.name annotations).
	Tags(ctx context.Context) ([]domain.Tag, error)

	// ResolveTag returns the manifest (or index) blob the tag points to.
	// Returns (nil, nil) if the tag is absent from this layout.
	ResolveTag(ctx context.Context, tag domain.Tag) (*domain.FetchManifestOutput, error)

	// ReadManifestByDigest returns the manifest/index blob with the given
	// digest, but only if it is reachable from index.json (directly or via a
	// nested index). Returns (nil, nil) if unreachable here.
	ReadManifestByDigest(
		ctx context.Context,
		digest domain.Digest,
	) (*domain.FetchManifestOutput, error)

	// OpenBlob returns a reader over the blob with the given digest, but only
	// if it is authorized (referenced as config/layer/subject by a reachable
	// image manifest). Returns (nil, nil) if not authorized here.
	OpenBlob(ctx context.Context, digest domain.Digest) (io.ReadSeekCloser, error)
}

// LayoutWalker yields one Layout per (solution, version) candidate that
// contains the image, in priority order.
type LayoutWalker interface {
	WalkLayouts(ctx context.Context, imageName domain.ImageName) iter.Seq2[Layout, error]
}
