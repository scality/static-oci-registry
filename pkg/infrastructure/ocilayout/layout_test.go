package ocilayout_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
)

// --- shared synthetic OCI-layout test builder (used across reader tests) ---

type layoutBuilder struct {
	root     string // os.Root base (the FS root)
	imageDir string // <solution>/<version>/<image> relative to root
}

func newLayoutBuilder(t *testing.T, imageDir string) *layoutBuilder { //nolint:unparam // imageDir will vary across future tests
	t.Helper()
	root := t.TempDir()

	full := filepath.Join(root, filepath.FromSlash(imageDir))
	if err := os.MkdirAll(filepath.Join(full, "blobs", "sha256"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(full, "oci-layout"),
		[]byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	return &layoutBuilder{root: root, imageDir: imageDir}
}

// putBlob writes content under blobs/sha256/<hex> and returns its digest.
func (b *layoutBuilder) putBlob(t *testing.T, content []byte) domain.Digest {
	t.Helper()

	sum := sha256.Sum256(content)
	hex := fmt.Sprintf("%x", sum[:])

	path := filepath.Join(b.root, filepath.FromSlash(b.imageDir), "blobs", "sha256", hex)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	return domain.Digest("sha256:" + hex)
}

// putJSON marshals v, stores it as a blob, and returns (digest, size).
func (b *layoutBuilder) putJSON(t *testing.T, v any) (domain.Digest, int64) {
	t.Helper()

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return b.putBlob(t, raw), int64(len(raw))
}

// writeIndex writes index.json from the given descriptors.
func (b *layoutBuilder) writeIndex(t *testing.T, manifests []domain.ManifestDescriptor) {
	t.Helper()

	idx := domain.Index{SchemaVersion: 2, MediaType: domain.MediaTypeOCIImageIndex, Manifests: manifests}

	raw, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(b.root, filepath.FromSlash(b.imageDir), "index.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (b *layoutBuilder) layout(t *testing.T) *ocilayout.Layout {
	t.Helper()

	osRoot, err := os.OpenRoot(b.root)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = osRoot.Close() })

	return ocilayout.NewLayout(slog.New(slog.DiscardHandler), osRoot, b.imageDir)
}

// a tiny valid image manifest (config + one layer).
func imageManifest(config, layer domain.Digest) domain.Manifest {
	return domain.Manifest{
		SchemaVersion: 2,
		MediaType:     domain.MediaTypeOCIImageManifest,
		Config:        &domain.ManifestDescriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: config, Size: 1},
		Layers:        []domain.ManifestDescriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layer, Size: 1}},
	}
}

// --- Tags ---

func TestLayoutTags(t *testing.T) {
	b := newLayoutBuilder(t, "sol/1.0.0/img")
	cfg := b.putBlob(t, []byte("config"))
	layer := b.putBlob(t, []byte("layer"))
	mDigest, mSize := b.putJSON(t, imageManifest(cfg, layer))
	b.writeIndex(t, []domain.ManifestDescriptor{
		{
			MediaType: domain.MediaTypeOCIImageManifest, Digest: mDigest, Size: mSize,
			Annotations: map[string]string{domain.RefNameAnnotation: "3.22"},
		},
		{MediaType: domain.MediaTypeOCIImageManifest, Digest: mDigest, Size: mSize}, // untagged → ignored
	})

	tags, err := b.layout(t).Tags(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) != 1 || tags[0] != domain.Tag("3.22") {
		t.Fatalf("got tags %v, want [3.22]", tags)
	}

	_ = io.Discard
}

func TestLayoutResolveTag(t *testing.T) {
	b := newLayoutBuilder(t, "sol/1.0.0/img")
	cfg := b.putBlob(t, []byte("config"))
	layer := b.putBlob(t, []byte("layer"))
	manifest := imageManifest(cfg, layer)
	mDigest, mSize := b.putJSON(t, manifest)
	b.writeIndex(t, []domain.ManifestDescriptor{
		{
			MediaType: domain.MediaTypeOCIImageManifest, Digest: mDigest, Size: mSize,
			Annotations: map[string]string{domain.RefNameAnnotation: "3.22"},
		},
	})
	l := b.layout(t)

	out, err := l.ResolveTag(context.Background(), domain.Tag("3.22"))
	if err != nil {
		t.Fatal(err)
	}

	if out == nil {
		t.Fatal("expected a result for tag 3.22")
	}

	if out.MediaType != domain.MediaTypeOCIImageManifest || out.ContentDigest != mDigest {
		t.Fatalf("unexpected output: %+v", out)
	}

	absent, err := l.ResolveTag(context.Background(), domain.Tag("nope"))
	if err != nil {
		t.Fatal(err)
	}

	if absent != nil {
		t.Fatalf("expected nil for absent tag, got %+v", absent)
	}
}

func TestLayoutReadManifestByDigest_MultiArch(t *testing.T) {
	b := newLayoutBuilder(t, "sol/1.0.0/img")

	// two per-platform image manifests
	cfgA := b.putBlob(t, []byte("configA"))
	layerA := b.putBlob(t, []byte("layerA"))
	mA, sizeA := b.putJSON(t, imageManifest(cfgA, layerA))

	cfgB := b.putBlob(t, []byte("configB"))
	layerB := b.putBlob(t, []byte("layerB"))
	mB, sizeB := b.putJSON(t, imageManifest(cfgB, layerB))

	// an image index referencing both, stored as a blob
	subIndex := domain.Index{
		SchemaVersion: 2, MediaType: domain.MediaTypeOCIImageIndex,
		Manifests: []domain.ManifestDescriptor{
			{MediaType: domain.MediaTypeOCIImageManifest, Digest: mA, Size: sizeA},
			{MediaType: domain.MediaTypeOCIImageManifest, Digest: mB, Size: sizeB},
		},
	}
	idxDigest, idxSize := b.putJSON(t, subIndex)

	// top-level index.json: tag points at the index blob
	b.writeIndex(t, []domain.ManifestDescriptor{
		{
			MediaType: domain.MediaTypeOCIImageIndex, Digest: idxDigest, Size: idxSize,
			Annotations: map[string]string{domain.RefNameAnnotation: "3.22"},
		},
	})
	l := b.layout(t)
	ctx := context.Background()

	// the index itself is reachable by digest
	if out, err := l.ReadManifestByDigest(ctx, idxDigest); err != nil || out == nil {
		t.Fatalf("index digest not reachable: out=%v err=%v", out, err)
	}
	// a per-platform sub-manifest is reachable by digest
	out, err := l.ReadManifestByDigest(ctx, mB)
	if err != nil || out == nil {
		t.Fatalf("sub-manifest not reachable: out=%v err=%v", out, err)
	}

	if out.MediaType != domain.MediaTypeOCIImageManifest || out.ContentDigest != mB {
		t.Fatalf("unexpected sub-manifest output: %+v", out)
	}
	// a random digest is not reachable
	absent := domain.Digest("sha256:" + "00000000000000000000000000000000000000000000000000000000000000aa")
	if out, err := l.ReadManifestByDigest(ctx, absent); err != nil || out != nil {
		t.Fatalf("unexpected reachable: out=%v err=%v", out, err)
	}
}

func TestLayoutReachableCycleGuard(t *testing.T) {
	// Content addressing makes true cycles impossible to construct, so assert
	// that duplicate descriptors terminate and dedupe rather than loop.
	b := newLayoutBuilder(t, "sol/1.0.0/img")
	cfg := b.putBlob(t, []byte("config"))
	layer := b.putBlob(t, []byte("layer"))
	m, size := b.putJSON(t, imageManifest(cfg, layer))
	b.writeIndex(t, []domain.ManifestDescriptor{
		{MediaType: domain.MediaTypeOCIImageManifest, Digest: m, Size: size},
		{MediaType: domain.MediaTypeOCIImageManifest, Digest: m, Size: size}, // duplicate
	})

	out, err := b.layout(t).ReadManifestByDigest(context.Background(), m)
	if err != nil || out == nil {
		t.Fatalf("expected reachable manifest, got out=%v err=%v", out, err)
	}
}
