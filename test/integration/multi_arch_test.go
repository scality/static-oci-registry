package integration

import (
	"crypto/tls"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("Multi-arch pull", Ordered, func() {
	var (
		re     *utils.RegistryEntry
		client *http.Client
	)

	const tag = "3.22"

	BeforeAll(func() {
		re = &utils.RegistryEntry{
			Solution: "multi-arch-solution",
			Version:  "v9.0.0",
			Image:    "docker.io/library/alpine",
			Tag:      tag,
		}
		suite.FetchImage(re)

		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	AfterAll(func() { suite.ClearImage(re) })

	It("serves the index at the tag, a sub-manifest by digest, and authorizes its blobs", func() {
		// Step 1: GET /v2/<name>/manifests/<tag> returns the image index.
		// The on-disk tag descriptor for a multi-arch image is an image index entry.
		tagDesc := onDiskTagDescriptor(re, tag)
		Expect(domain.IsImageIndexMediaType(tagDesc.MediaType)).To(BeTrue(),
			"expected an image index media type for tag %s, got %s", tag, tagDesc.MediaType)

		req := initRequest(string(re.Image), "/manifests/"+tag, nil)
		resp, body := execRequestRaw(client, req)

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get("Content-Type")).To(Equal(tagDesc.MediaType))
		Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(tagDesc.Digest.String()))

		indexBytes := onDiskBlob(re, tagDesc.Digest)
		Expect(body).To(BeEquivalentTo(indexBytes))

		// Step 2: Resolve a per-platform sub-manifest digest and fetch it by digest.
		_, subDigest := onDiskImageManifest(re, tag)

		req2 := initRequest(string(re.Image), "/manifests/"+string(subDigest), nil)
		resp2, body2 := execRequestRaw(client, req2)

		Expect(resp2.StatusCode).To(Equal(http.StatusOK))
		Expect(resp2.Header.Get("Docker-Content-Digest")).To(Equal(subDigest.String()))

		subManifestBytes := onDiskBlob(re, subDigest)
		Expect(body2).To(BeEquivalentTo(subManifestBytes))

		// Step 3: A config blob of the sub-manifest is served.
		subManifest, _ := onDiskImageManifest(re, tag)
		configDigest := subManifest.Config.Digest

		req3 := initRequest(string(re.Image), "/blobs/"+string(configDigest), nil)
		resp3, _ := execRequestRaw(client, req3)

		Expect(resp3.StatusCode).To(Equal(http.StatusOK))

		// Step 4: An unreferenced/bogus blob digest returns 404 BLOB_UNKNOWN.
		bogusDigest := sha256Digest([]byte("this blob does not exist"))

		req4 := initRequest(string(re.Image), "/blobs/"+string(bogusDigest), nil)
		resp4, body4 := execRequest(client, req4)

		Expect(resp4.StatusCode).To(Equal(http.StatusNotFound))
		checkErrorResponse(body4, ocierrors.BlobUnknown)
	})
})
