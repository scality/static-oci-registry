package integration

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/test/utils"
)

// copyTagDir duplicates an existing tag directory under a new tag name in the
// same image dir, so the same image+tag can exist in multiple solution-versions
// with different content.
func copyTagDir(re *utils.RegistryEntry, newTag string) {
	src := re.FullPath(suite.FsRoot)
	dst := filepath.Join(re.ImagePath(suite.FsRoot), newTag)
	// nolint: gosec // G204: this is acceptable since it's for tests only
	Expect(exec.Command("cp", "-r", src, dst).Run()).To(Succeed())
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

		suite.FetchImage(reOld)
		suite.FetchImage(reNew)

		// Expose both under the same "latest" tag inside each version dir.
		copyTagDir(reOld, sharedTag)
		copyTagDir(reNew, sharedTag)

		// Expected truth: the newest solution-version (v2.0.0).
		newManifestBytes = readOnDiskManifest(reNew)

		var m domain.Manifest
		Expect(json.Unmarshal(newManifestBytes, &m)).To(Succeed())
		newManifestType = m.MediaType
		Expect(m.Config).NotTo(BeNil())
		newConfigDigest = m.Config.Digest
		newConfigBytes = readOnDiskBlob(reNew, newConfigDigest)

		// Sanity check: the two versions really do have different content,
		// otherwise this test would pass trivially regardless of ordering.
		oldManifestBytes := readOnDiskManifest(reOld)
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
			oldManifest := parseOnDiskManifest(reOld)
			Expect(oldManifest.Config).NotTo(BeNil())
			oldConfigDigest := oldManifest.Config.Digest

			// If old and new happened to share the config blob (they shouldn't,
			// given the VERSION label differs), skip the assertion.
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
		It("has the shared tag in both solution-versions", func() {
			oldLatest := filepath.Join(reOld.ImagePath(suite.FsRoot), sharedTag, "manifest.json")
			newLatest := filepath.Join(reNew.ImagePath(suite.FsRoot), sharedTag, "manifest.json")

			Expect(oldLatest).To(BeAnExistingFile())
			Expect(newLatest).To(BeAnExistingFile())

			oldBytes, err := os.ReadFile(oldLatest)
			Expect(err).NotTo(HaveOccurred())
			newBytes, err := os.ReadFile(newLatest)
			Expect(err).NotTo(HaveOccurred())

			Expect(oldBytes).NotTo(BeEquivalentTo(newBytes),
				"the two shared-tag manifests must differ; otherwise the test cannot detect ordering")
		})
	})
})
