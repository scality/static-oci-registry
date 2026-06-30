package layoutwalker_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/layoutwalker"
)

// writeLayout creates a minimal OCI Image Layout at <root>/<sol>/<ver>/<image>
// with a single tagged entry in index.json - enough for the image finder to
// recognise it and for the yielded Layout.Tags to return the tag.
func writeLayout(t *testing.T, root, sol, ver, image, tag string) {
	t.Helper()

	dir := filepath.Join(root, sol, ver, image)
	if err := os.MkdirAll(filepath.Join(dir, "blobs", "sha256"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "oci-layout"),
		[]byte(`{"imageLayoutVersion":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := domain.Index{
		SchemaVersion: 2,
		MediaType:     domain.MediaTypeOCIImageIndex,
		Manifests: []domain.ManifestDescriptor{{
			MediaType:   domain.MediaTypeOCIImageManifest,
			Digest:      "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			Size:        1,
			Annotations: map[string]string{domain.RefNameAnnotation: tag},
		}},
	}

	raw, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "index.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newWalker(t *testing.T, fsRoot string) *layoutwalker.FileSystem {
	t.Helper()

	osRoot, err := os.OpenRoot(fsRoot)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = osRoot.Close() })

	logger := slog.New(slog.DiscardHandler)

	finder, err := imagefinder.NewFileSystem(logger, osRoot)
	if err != nil {
		t.Fatal(err)
	}

	w, err := layoutwalker.NewFileSystem(logger, finder, osRoot)
	if err != nil {
		t.Fatal(err)
	}

	return w
}

func TestWalkLayoutsYieldsCandidateLayouts(t *testing.T) {
	root := t.TempDir()
	writeLayout(t, root, "sol-a", "1.0.0", "img", "3.22")

	w := newWalker(t, root)

	var got []domain.Tag

	for layout, err := range w.WalkLayouts(context.Background(), domain.ImageName("img")) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if layout == nil {
			t.Fatal("yielded a nil layout")
		}

		tags, err := layout.Tags(context.Background())
		if err != nil {
			t.Fatalf("layout.Tags failed: %v", err)
		}

		got = append(got, tags...)
	}

	if len(got) != 1 || got[0] != domain.Tag("3.22") {
		t.Fatalf("expected [3.22], got %v", got)
	}
}

func TestWalkLayoutsImageNotFound(t *testing.T) {
	w := newWalker(t, t.TempDir()) // empty root: no images

	var gotErr error

	for _, err := range w.WalkLayouts(context.Background(), domain.ImageName("absent")) {
		gotErr = err
	}

	if gotErr == nil {
		t.Fatal("expected an error for a missing image")
	}
}
