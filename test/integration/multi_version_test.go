package integration

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/test/utils"
)

// addTagAlias writes a new entry into an OCI Image Layout's index.json that
// points the given alias tag at the same descriptor as srcTag. This lets the
// same layout expose a "latest" alias alongside the canonical version tag
// without re-downloading any blobs.
func addTagAlias(re *utils.RegistryEntry, srcTag, aliasTag string) {
	indexPath := filepath.Join(re.ImagePath(suite.FsRoot), "index.json")

	raw, err := os.ReadFile(indexPath)
	Expect(err).NotTo(HaveOccurred())

	var idx domain.Index
	Expect(json.Unmarshal(raw, &idx)).To(Succeed())

	// Find the source descriptor.
	var srcDesc domain.ManifestDescriptor

	found := false

	for _, m := range idx.Manifests {
		if m.Annotations[domain.RefNameAnnotation] == srcTag {
			srcDesc = m
			found = true

			break
		}
	}

	Expect(found).To(BeTrue(), "source tag %q not found in index.json", srcTag)

	// Clone the descriptor and update the ref.name annotation to the alias.
	aliasDesc := srcDesc
	aliasAnnotations := make(map[string]string, len(srcDesc.Annotations))

	for k, v := range srcDesc.Annotations {
		aliasAnnotations[k] = v
	}

	aliasAnnotations[domain.RefNameAnnotation] = aliasTag
	aliasDesc.Annotations = aliasAnnotations

	idx.Manifests = append(idx.Manifests, aliasDesc)

	out, err := json.Marshal(idx)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(indexPath, out, 0o600)).To(Succeed())
}

var _ = Describe("Multi-version solution: latest tag resolution", Ordered, func() {
	// Same solution + image present in two versions, both exposing the same
	// "latest" tag with different content. The registry must serve the manifest
	// (and its blobs) from the highest semver solution-version.
	const (
		solution                   = "multi-version-solution"
		image     domain.ImageName = "docker.io/library/alpine"
		oldTag                     = "3.20.0"
		newTag                     = "3.22.2"
		sharedTag                  = "latest"
	)

	var (
		client *http.Client

		reOld *utils.RegistryEntry // v1.0.0, content built from oldTag
		reNew *utils.RegistryEntry // v2.0.0, content built from newTag

		newManifestBytes []byte
		newManifestType  string
		newConfigDigest  domain.Digest
		newConfigBytes   []byte
	)

	BeforeAll(func() {
		reOld = &utils.RegistryEntry{
			Solution: solution,
			Version:  "v1.0.0",
			Image:    image,
			Tag:      oldTag,
		}
		reNew = &utils.RegistryEntry{
			Solution: solution,
			Version:  "v2.0.0",
			Image:    image,
			Tag:      newTag,
		}

		suite.BuildImage(reOld)
		suite.BuildImage(reNew)

		// Expose both under the same "latest" tag by aliasing in each layout's index.json.
		addTagAlias(reOld, oldTag, sharedTag)
		addTagAlias(reNew, newTag, sharedTag)

		// Expected truth: the newest solution-version (v2.0.0).
		// For a multi-arch image, GET /manifests/<tag> returns the index descriptor.
		newDesc := onDiskTagDescriptor(reNew, sharedTag)
		newManifestBytes = onDiskBlob(reNew, newDesc.Digest)
		newManifestType = newDesc.MediaType

		// Descend one level to find the concrete image manifest for config/layer digests.
		newImageManifest, _ := onDiskImageManifest(reNew, sharedTag)
		Expect(newImageManifest.Config).NotTo(BeNil())
		newConfigDigest = newImageManifest.Config.Digest
		newConfigBytes = onDiskBlob(reNew, newConfigDigest)

		// Sanity check: the two versions really do have different content,
		// otherwise this test would pass trivially regardless of ordering.
		oldDesc := onDiskTagDescriptor(reOld, sharedTag)
		oldManifestBytes := onDiskBlob(reOld, oldDesc.Digest)
		Expect(oldManifestBytes).NotTo(BeEquivalentTo(newManifestBytes),
			"fixture bug: old and new manifests must differ for this test to be meaningful")
	})

	AfterAll(func() {
		suite.ClearImage(reOld)
		suite.ClearImage(reNew)
	})

	BeforeEach(func() {
		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	When("fetching the manifest by the shared tag", func() {
		It("returns the manifest from the highest solution-version", func() {
			req := initRequest(string(image), "/manifests/"+sharedTag, nil)

			resp, body := execRequestRaw(client, req)

			wantDigest := sha256Digest(newManifestBytes)
			checkManifestResponse(resp, body, newManifestBytes, newManifestType, wantDigest)
		})
	})

	When("HEADing the manifest by the shared tag", func() {
		It("advertises the digest of the highest solution-version's manifest", func() {
			req := initRequestWithMethod(http.MethodHead, string(image), "/manifests/"+sharedTag, nil)

			resp, body := execRequestRaw(client, req)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Docker-Content-Digest")).To(
				Equal(sha256Digest(newManifestBytes).String()),
			)
			Expect(resp.Header.Get("Content-Type")).To(Equal(newManifestType))
			Expect(body).To(BeEmpty())
		})
	})

	When("pulling the config blob referenced by the shared-tag manifest", func() {
		It("returns the blob from the highest solution-version", func() {
			req := initRequest(string(image), "/blobs/"+string(newConfigDigest), nil)

			resp, body := execRequestRaw(client, req)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(newConfigDigest.String()))
			Expect(resp.Header.Get("Content-Length")).To(Equal(strconv.Itoa(len(newConfigBytes))))
			Expect(body).To(BeEquivalentTo(newConfigBytes))
		})
	})

	When("the old version's config blob is queried via the image (not the tag)", func() {
		It("still resolves: blob lookup walks all candidates, ordering is only tag-resolution", func() {
			// Authorization for blobs walks every candidate manifest, so the
			// old version's config digest is still serveable. This guards
			// against a regression where ordering changes accidentally
			// short-circuit the blob-authorization walk.
			oldImageManifest, _ := onDiskImageManifest(reOld, sharedTag)
			Expect(oldImageManifest.Config).NotTo(BeNil())
			oldConfigDigest := oldImageManifest.Config.Digest

			// If old and new happened to share the config blob, skip the assertion.
			if oldConfigDigest == newConfigDigest {
				Skip("old and new config digests are identical; nothing to compare")
			}

			req := initRequest(string(image), "/blobs/"+string(oldConfigDigest), nil)

			resp, _ := execRequestRaw(client, req)

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(oldConfigDigest.String()))
		})
	})

	When("verifying the on-disk layout matches the intent", func() {
		It("has the shared tag in both solution-version index.json files", func() {
			// Verify addTagAlias worked: both layouts expose sharedTag.
			oldDesc := onDiskTagDescriptor(reOld, sharedTag)
			newDesc := onDiskTagDescriptor(reNew, sharedTag)

			// They must point to different digests (different image versions).
			Expect(oldDesc.Digest).NotTo(Equal(newDesc.Digest),
				"the two shared-tag descriptors must differ; otherwise the test cannot detect ordering")
		})
	})
})
