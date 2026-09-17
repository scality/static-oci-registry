package ocilayout

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

const (
	indexFileName = "index.json"
	blobsDirName  = "blobs"
	// maxIndexDepth bounds nested-index traversal (image-spec allows nesting).
	maxIndexDepth = 4
)

// Layout is a request-scoped reader over one image's OCI Image Layout. It
// caches the parsed index.json for the lifetime of this value only (one
// request); it never caches across requests.
type Layout struct {
	logger    *slog.Logger
	root      *os.Root
	path      string // "<solution>/<version>/<image>" relative to root
	solution  domain.SolutionVersion
	imageName domain.ImageName

	index *domain.Index // lazily loaded, request-scoped cache
}

var _ service.Layout = (*Layout)(nil)

// NewLayout builds a Layout that reads the OCI image layout at
// "<sv.Solution>/<sv.Version>/<imageName>" beneath root. The (solution,
// version) tuple is retained separately so SolutionVersion() can return it
// without re-parsing the path.
func NewLayout(
	logger *slog.Logger,
	root *os.Root,
	sv domain.SolutionVersion,
	imageName domain.ImageName,
) *Layout {
	path := strings.Join([]string{sv.Solution, sv.Version, imageName.String()}, "/")

	return &Layout{
		logger:    logger.With(slog.String("ocilayout", path)),
		root:      root,
		path:      path,
		solution:  sv,
		imageName: imageName,
	}
}

func (l *Layout) Location() string {
	return l.path
}

// SolutionVersion returns the (solution, version) tuple this layout was built from.
func (l *Layout) SolutionVersion() domain.SolutionVersion {
	return l.solution
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

func (l *Layout) ResolveTag(
	ctx context.Context, tag domain.Tag,
) (*domain.FetchManifestOutput, error) {
	idx, err := l.readIndex(ctx)
	if err != nil {
		return nil, err
	}

	for _, m := range idx.Manifests {
		if m.Annotations[domain.RefNameAnnotation] != tag.String() {
			continue
		}

		raw, err := l.readBlob(m.Digest)
		if err != nil {
			return nil, err
		}

		return &domain.FetchManifestOutput{
			MediaType:     m.MediaType,
			ContentDigest: m.Digest,
			ManifestBytes: raw,
		}, nil
	}

	return nil, nil //nolint:nilnil // (nil,nil) means "tag absent here, try next candidate"
}

// manifestNode is an entry in the reachableManifests traversal queue: a
// descriptor together with its nesting depth in the index graph.
type manifestNode struct {
	desc  domain.ManifestDescriptor
	depth int
}

// expandIndex reads the nested index at n.desc.Digest and returns its child
// descriptors as new queue nodes at depth n.depth+1. The nested index is
// validated like the top-level one (readIndex). Read, parse, and validation
// errors are soft-failed: logged as warnings and nil returned so one bad blob
// never aborts the walk.
func (l *Layout) expandIndex(ctx context.Context, n manifestNode) []manifestNode {
	if n.depth >= maxIndexDepth {
		l.logger.WarnContext(ctx, "nested index too deep, not descending",
			slog.String("digest", n.desc.Digest.String()), slog.Int("depth", n.depth))

		return nil
	}

	raw, err := l.readBlob(n.desc.Digest)
	if err != nil {
		l.logger.WarnContext(ctx, "failed to read nested index blob, skipping",
			slog.String("digest", n.desc.Digest.String()), slog.Any("error", err))

		return nil
	}

	var sub domain.Index
	if err := json.Unmarshal(raw, &sub); err != nil {
		l.logger.WarnContext(ctx, "failed to unmarshal nested index, skipping",
			slog.String("digest", n.desc.Digest.String()), slog.Any("error", err))

		return nil
	}

	if err := sub.Validate(); err != nil {
		l.logger.WarnContext(ctx, "invalid nested index, skipping",
			slog.String("digest", n.desc.Digest.String()), slog.Any("error", err))

		return nil
	}

	children := make([]manifestNode, len(sub.Manifests))
	for i, m := range sub.Manifests {
		children[i] = manifestNode{desc: m, depth: n.depth + 1}
	}

	return children
}

// reachableManifests returns every manifest/index descriptor reachable from
// index.json (entries plus the contents of nested indexes). Traversal is
// bounded by maxIndexDepth and guarded against cycles. Unreadable or malformed
// nested indexes are logged and skipped (soft-fail).
func (l *Layout) reachableManifests(ctx context.Context) ([]domain.ManifestDescriptor, error) {
	idx, err := l.readIndex(ctx)
	if err != nil {
		return nil, err
	}

	queue := make([]manifestNode, 0, len(idx.Manifests))
	for _, m := range idx.Manifests {
		queue = append(queue, manifestNode{desc: m, depth: 0})
	}

	visited := make(map[domain.Digest]struct{})
	out := make([]domain.ManifestDescriptor, 0, len(queue))

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]

		if _, seen := visited[n.desc.Digest]; seen {
			continue
		}

		visited[n.desc.Digest] = struct{}{}
		out = append(out, n.desc)

		if domain.IsImageIndexMediaType(n.desc.MediaType) {
			queue = append(queue, l.expandIndex(ctx, n)...)
		}
	}

	return out, nil
}

func (l *Layout) ReadManifestByDigest(
	ctx context.Context, digest domain.Digest,
) (*domain.FetchManifestOutput, error) {
	descs, err := l.reachableManifests(ctx)
	if err != nil {
		return nil, err
	}

	for _, d := range descs {
		if d.Digest != digest {
			continue
		}

		raw, err := l.readBlob(digest)
		if err != nil {
			return nil, err
		}

		return &domain.FetchManifestOutput{
			MediaType:     d.MediaType,
			ContentDigest: digest,
			ManifestBytes: raw,
		}, nil
	}

	return nil, nil //nolint:nilnil // (nil,nil) means "unreachable here, try next candidate"
}

// addManifestBlobs reads and parses the image manifest at d.Digest and adds
// its config, layer, and subject digests to set. Read/unmarshal errors are
// soft-failed: logged and skipped so one bad blob never aborts the walk.
func (l *Layout) addManifestBlobs(
	ctx context.Context, d domain.ManifestDescriptor, set map[domain.Digest]struct{},
) {
	raw, err := l.readBlob(d.Digest)
	if err != nil {
		l.logger.WarnContext(ctx, "failed to read reachable manifest blob, skipping",
			slog.String("digest", d.Digest.String()), slog.Any("error", err))

		return
	}

	var m domain.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		l.logger.WarnContext(ctx, "failed to unmarshal reachable manifest, skipping",
			slog.String("digest", d.Digest.String()), slog.Any("error", err))

		return
	}

	if m.Config != nil {
		set[m.Config.Digest] = struct{}{}
	}

	for _, layer := range m.Layers {
		set[layer.Digest] = struct{}{}
	}

	if m.Subject != nil {
		set[m.Subject.Digest] = struct{}{}
	}
}

// authorizedBlobs returns the set of blob digests that may be served: the
// config, layers, and subject of every reachable image manifest. Manifest/
// index blobs are intentionally excluded (served via the manifest endpoint).
func (l *Layout) authorizedBlobs(ctx context.Context) (map[domain.Digest]struct{}, error) {
	descs, err := l.reachableManifests(ctx)
	if err != nil {
		return nil, err
	}

	set := make(map[domain.Digest]struct{})

	for _, d := range descs {
		// Skip indexes: they carry no blobs of their own and their child
		// manifests are already in descs. Everything else is treated as an
		// image manifest and mined for its config/layers/subject.
		if domain.IsImageIndexMediaType(d.MediaType) {
			continue
		}

		l.addManifestBlobs(ctx, d, set)
	}

	return set, nil
}

func (l *Layout) OpenBlob(ctx context.Context, digest domain.Digest) (io.ReadSeekCloser, error) {
	path, err := l.blobPath(digest)
	if err != nil {
		return nil, err
	}

	// Cheap existence check first: if the blob isn't in this layout's store,
	// there is no point computing the authorized set (which parses manifests) —
	// let the caller try the next candidate.
	info, err := l.root.Stat(path)
	if err != nil || info.IsDir() {
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			l.logger.WarnContext(ctx, "failed to stat blob, skipping",
				slog.String("path", path), slog.Any("error", err))
		}

		return nil, nil //nolint:nilnil // not present here, try next candidate
	}

	authorized, err := l.authorizedBlobs(ctx)
	if err != nil {
		return nil, err
	}

	if _, ok := authorized[digest]; !ok {
		return nil, nil //nolint:nilnil // present but unreferenced — not authorized here
	}

	file, err := l.root.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to open authorized blob"))
	}

	return file, nil
}
