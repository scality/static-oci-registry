package domain_test

import (
	"testing"

	"github.com/scality/static-oci-registry/pkg/domain"
)

func TestMediaTypeClassifiers(t *testing.T) {
	cases := []struct {
		mt      string
		isIndex bool
	}{
		{domain.MediaTypeOCIImageManifest, false},
		{domain.MediaTypeOCIImageIndex, true},
		{domain.MediaTypeDockerManifestList, true},
		{"application/octet-stream", false},
		{"", false},
	}
	for _, c := range cases {
		if got := domain.IsImageIndexMediaType(c.mt); got != c.isIndex {
			t.Errorf("IsImageIndexMediaType(%q) = %v, want %v", c.mt, got, c.isIndex)
		}
	}
}
