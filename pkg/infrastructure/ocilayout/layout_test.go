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

func newLayoutBuilder(t *testing.T, imageDir string) *layoutBuilder {
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
