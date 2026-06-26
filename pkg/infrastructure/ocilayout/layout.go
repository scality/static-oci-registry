package ocilayout

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
)

const (
	indexFileName = "index.json"
	blobsDirName  = "blobs"
	// maxIndexDepth bounds nested-index traversal (image-spec allows nesting).
	maxIndexDepth = 4 //nolint:unused // used by Tasks 5-7 (ResolveTag, ReadManifestByDigest, OpenBlob)
)

// Layout is a request-scoped reader over one image's OCI Image Layout. It
// caches the parsed index.json for the lifetime of this value only (one
// request); it never caches across requests.
type Layout struct {
	logger *slog.Logger
	root   *os.Root
	path   string // "<solution>/<version>/<image>" relative to root

	index *domain.Index // lazily loaded, request-scoped cache
}

func NewLayout(logger *slog.Logger, root *os.Root, path string) *Layout {
	return &Layout{
		logger: logger.With(slog.String("ocilayout", path)),
		root:   root,
		path:   path,
	}
}

func (l *Layout) Location() string {
	return l.path
}

func (l *Layout) readIndex(_ context.Context) (*domain.Index, error) {
	if l.index != nil {
		return l.index, nil
	}

	raw, err := l.root.ReadFile(l.path + "/" + indexFileName)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to read index.json"))
	}

	var idx domain.Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to unmarshal index.json"))
	}

	if err := idx.Validate(); err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("invalid index.json"))
	}

	l.index = &idx

	return l.index, nil
}

func (l *Layout) Tags(ctx context.Context) ([]domain.Tag, error) {
	idx, err := l.readIndex(ctx)
	if err != nil {
		return nil, err
	}

	var tags []domain.Tag

	for _, m := range idx.Manifests {
		name, ok := m.Annotations[domain.RefNameAnnotation]
		if !ok || name == "" {
			continue
		}

		tag := domain.Tag(name)
		if err := tag.Validate(); err != nil {
			l.logger.WarnContext(ctx, "invalid tag annotation in index.json, skipping",
				slog.String("tag", name), slog.Any("error", err))

			continue
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

// blobPath returns "<path>/blobs/<algo>/<encoded>" for the given digest.
//
//nolint:unused // used by Tasks 6-7 (ReadManifestByDigest, OpenBlob)
func (l *Layout) blobPath(digest domain.Digest) (string, error) {
	algo, err := digest.Algorithm()
	if err != nil {
		return "", errors.Wrap(err, errors.WithDetail("failed to get digest algorithm"))
	}

	encoded, err := digest.Encoded()
	if err != nil {
		return "", errors.Wrap(err, errors.WithDetail("failed to get digest encoded value"))
	}

	return strings.Join([]string{l.path, blobsDirName, algo, encoded}, "/"), nil
}

//nolint:unused // used by Tasks 6-7 (ReadManifestByDigest, OpenBlob)
func (l *Layout) readBlob(digest domain.Digest) ([]byte, error) {
	path, err := l.blobPath(digest)
	if err != nil {
		return nil, err
	}

	raw, err := l.root.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to read blob"))
	}

	return raw, nil
}
