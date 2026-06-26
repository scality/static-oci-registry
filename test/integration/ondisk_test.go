package integration

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/test/utils"
)

// onDiskTagDescriptor returns the index.json entry (digest + mediaType) that a
// tag points to. For a multi-arch image this is the image index descriptor.
func onDiskTagDescriptor(re *utils.RegistryEntry, tag string) domain.ManifestDescriptor {
	raw, err := os.ReadFile(filepath.Join(re.ImagePath(suite.FsRoot), "index.json"))
	Expect(err).NotTo(HaveOccurred())

	var idx domain.Index
	Expect(json.Unmarshal(raw, &idx)).To(Succeed())

	for _, m := range idx.Manifests {
		if m.Annotations[domain.RefNameAnnotation] == tag {
			return m
		}
	}

	Fail("tag not found in index.json: " + tag)

	return domain.ManifestDescriptor{}
}

// onDiskBlob reads blobs/<algo>/<encoded> for the given digest.
func onDiskBlob(re *utils.RegistryEntry, dgst domain.Digest) []byte {
	algo, err := dgst.Algorithm()
	Expect(err).NotTo(HaveOccurred())
	enc, err := dgst.Encoded()
	Expect(err).NotTo(HaveOccurred())

	raw, err := os.ReadFile(filepath.Join(re.ImagePath(suite.FsRoot), "blobs", algo, enc))
	Expect(err).NotTo(HaveOccurred())

	return raw
}

// onDiskImageManifest resolves a tag to a concrete image manifest, descending
// one level into an image index and picking the first entry. Returns the
// manifest and its digest.
//
//nolint:unparam // digest is returned for callers that may need it
func onDiskImageManifest(re *utils.RegistryEntry, tag string) (domain.Manifest, domain.Digest) {
	desc := onDiskTagDescriptor(re, tag)

	dgst := desc.Digest
	if domain.IsImageIndexMediaType(desc.MediaType) {
		var idx domain.Index
		Expect(json.Unmarshal(onDiskBlob(re, desc.Digest), &idx)).To(Succeed())
		Expect(idx.Manifests).NotTo(BeEmpty())
		dgst = idx.Manifests[0].Digest
	}

	var m domain.Manifest
	Expect(json.Unmarshal(onDiskBlob(re, dgst), &m)).To(Succeed())

	return m, dgst
}
