package domain_test

import (
	"testing"

	"github.com/scality/static-oci-registry/pkg/domain"
)

func validIndex() domain.Index {
	return domain.Index{
		SchemaVersion: 2,
		MediaType:     domain.MediaTypeOCIImageIndex,
		Manifests: []domain.ManifestDescriptor{
			{
				MediaType:   domain.MediaTypeOCIImageManifest,
				Digest:      "sha256:0000000000000000000000000000000000000000000000000000000000000001",
				Size:        10,
				Annotations: map[string]string{domain.RefNameAnnotation: "3.22"},
			},
		},
	}
}

func TestIndexValidate(t *testing.T) {
	if err := validIndex().Validate(); err != nil {
		t.Fatalf("valid index rejected: %v", err)
	}

	bad := validIndex()

	bad.SchemaVersion = 1
	if err := bad.Validate(); err == nil {
		t.Error("expected error for schemaVersion != 2")
	}

	bad = validIndex()

	bad.MediaType = domain.MediaTypeOCIImageManifest // not an index type
	if err := bad.Validate(); err == nil {
		t.Error("expected error for non-index mediaType")
	}

	bad = validIndex()

	bad.Manifests[0].Digest = "not-a-digest"
	if err := bad.Validate(); err == nil {
		t.Error("expected error for invalid descriptor digest")
	}
}
