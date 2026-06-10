package integration

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/test/utils"
)

// parseOnDiskManifest unmarshals the manifest.json written by skopeo so the
// tests can pick real digests dynamically instead of hardcoding them.
func parseOnDiskManifest(re *utils.RegistryEntry) domain.Manifest {
	bytes := readOnDiskManifest(re)

	var m domain.Manifest
	Expect(json.Unmarshal(bytes, &m)).To(Succeed())

	return m
}

// digestEncoded extracts the encoded portion of a digest. Tests construct
// digests from known-valid bytes (the on-disk manifest or hardcoded
// sha256/sha512 literals), so an error here is a fixture bug.
func digestEncoded(d domain.Digest) string {
	enc, err := d.Encoded()
	Expect(err).NotTo(HaveOccurred())

	return enc
}

// readOnDiskBlob reads the content-addressed blob file from the tag dir for
// byte-for-byte comparison against the HTTP response body.
func readOnDiskBlob(re *utils.RegistryEntry, dgst domain.Digest) []byte {
	bytes, err := os.ReadFile(filepath.Join(re.FullPath(suite.FsRoot), digestEncoded(dgst)))
	Expect(err).NotTo(HaveOccurred())

	return bytes
}

// checkBlobResponse asserts the registry returned the blob verbatim with
// the headers required by the OCI distribution-spec.
func checkBlobResponse(
	resp *http.Response,
	body []byte,
	wantBytes []byte,
	wantDigest domain.Digest,
) {
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(resp.Header.Get("Content-Type")).To(Equal("application/octet-stream"))
	Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(wantDigest.String()))
	Expect(resp.Header.Get("Content-Length")).To(Equal(strconv.Itoa(len(wantBytes))))
	Expect(body).To(BeEquivalentTo(wantBytes))
}

var _ = Describe("Pull Blob Integration", Ordered, func() {
	var client *http.Client

	BeforeEach(func() {
		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	Context("Pulling blobs via HTTPS in a healthy FS", Ordered, func() {
		solution := "pull-blob-solution"

		var (
			image           domain.ImageName = "docker.io/library/alpine"
			tag                              = "3.22.2"
			re              *utils.RegistryEntry
			configDigest    domain.Digest
			layerDigest     domain.Digest
			wantConfigBytes []byte
			wantLayerBytes  []byte
		)

		BeforeAll(func() {
			re = &utils.RegistryEntry{
				Solution: solution,
				Version:  "v1.0.0",
				Image:    image,
				Tag:      tag,
			}
			suite.FetchImage(re)

			m := parseOnDiskManifest(re)
			Expect(m.Config).NotTo(BeNil())
			Expect(m.Layers).NotTo(BeEmpty(),
				"test image must have at least one layer for the layer-blob spec")

			configDigest = m.Config.Digest
			layerDigest = m.Layers[0].Digest
			wantConfigBytes = readOnDiskBlob(re, configDigest)
			wantLayerBytes = readOnDiskBlob(re, layerDigest)
		})

		AfterAll(func() {
			suite.ClearImage(re)
		})

		Context("GET happy paths", func() {
			When("fetching the config blob with URL-encoded slashes in the image name", func() {
				It("returns the blob verbatim with correct headers", func() {
					req := initRequest(string(image), "/blobs/"+string(configDigest), nil)

					resp, body := execRequestRaw(client, req)

					checkBlobResponse(resp, body, wantConfigBytes, configDigest)
				})
			})

			When("fetching a layer blob with unencoded slashes in the image name", func() {
				It("returns the blob verbatim with correct headers", func() {
					url := "https://localhost" + cfg.HTTP.Addr + "/v2/" + string(image) +
						"/blobs/" + string(layerDigest)
					req, err := http.NewRequest(http.MethodGet, url, nil)
					Expect(err).NotTo(HaveOccurred())

					resp, body := execRequestRaw(client, req)

					checkBlobResponse(resp, body, wantLayerBytes, layerDigest)
				})
			})

			When("fetching the config blob with ns query param (containerd-style)", func() {
				It("returns the blob verbatim with correct headers", func() {
					// containerd splits "docker.io/library/alpine" into
					// ns=docker.io and image=library/alpine; the handler must rejoin them.
					req := initRequest(
						"library/alpine", "/blobs/"+string(configDigest), QueryParams{"ns": "docker.io"},
					)

					resp, body := execRequestRaw(client, req)

					checkBlobResponse(resp, body, wantConfigBytes, configDigest)
				})
			})
		})

		Context("GET range requests (handled by http.ServeContent)", func() {
			When("the client asks for a valid byte range", func() {
				It("returns 206 with Content-Range and just the requested bytes", func() {
					req := initRequest(string(image), "/blobs/"+string(configDigest), nil)
					req.Header.Set("Range", "bytes=0-15")

					resp, body := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusPartialContent))
					Expect(resp.Header.Get("Content-Range")).To(Equal(
						fmt.Sprintf("bytes 0-15/%d", len(wantConfigBytes)),
					))
					Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(configDigest.String()),
						"digest header must describe the full blob, not the partial range")
					Expect(body).To(BeEquivalentTo(wantConfigBytes[:16]))
				})
			})

			When("the client asks for a range past the end of the blob", func() {
				It("returns 416 Requested Range Not Satisfiable", func() {
					req := initRequest(string(image), "/blobs/"+string(configDigest), nil)
					req.Header.Set("Range", fmt.Sprintf("bytes=%d-", len(wantConfigBytes)+10))

					resp, _ := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusRequestedRangeNotSatisfiable))
				})
			})
		})

		Context("GET 404 error envelopes", func() {
			When("the digest is not referenced by any manifest", func() {
				It("should return a BLOB_UNKNOWN error", func() {
					ghost := domain.Digest(
						"sha256:0000000000000000000000000000000000000000000000000000000000000001",
					)
					req := initRequest(string(image), "/blobs/"+string(ghost), nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.BlobUnknown)
				})
			})

			When("the requested digest uses an unsupported algorithm", func() {
				It("should return a BLOB_UNKNOWN error", func() {
					// sha512: grammar-valid, never matches a sha256-stored blob
					unsupported := "sha512:" +
						"00000000000000000000000000000000000000000000000000000000000000000000" +
						"00000000000000000000000000000000000000000000000000000000000000"
					req := initRequest(string(image), "/blobs/"+unsupported, nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.BlobUnknown)
				})
			})

			When("the image does not exist", func() {
				It("should return a NAME_UNKNOWN error", func() {
					req := initRequest(string(image)+"-ghost", "/blobs/"+string(configDigest), nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.NameUnknown)
				})
			})
		})

		Context("GET malformed requests", func() {
			When("the image name is invalid", func() {
				It("should return a NAME_INVALID error", func() {
					req := initRequest("Docker/Library/Alpine", "/blobs/"+string(configDigest), nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.NameInvalid)
				})
			})

			When("the digest grammar is invalid", func() {
				It("should return a DIGEST_INVALID error", func() {
					req := initRequest(string(image), "/blobs/not-a-digest", nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.DigestInvalid)
				})
			})
		})

		Context("Security: manifest-authorized blob serving", func() {
			When("a file is present in the tag dir but not referenced by the manifest", func() {
				It("should return BLOB_UNKNOWN and never serve the file contents", func() {
					// Pick a sha256 that is grammar-valid but not in the manifest,
					// then drop a file at <tag>/<encoded> with marker contents.
					strayDigest := domain.Digest(
						"sha256:dead000000000000000000000000000000000000000000000000000000000000",
					)
					strayPath := filepath.Join(re.FullPath(suite.FsRoot), digestEncoded(strayDigest))
					strayContents := []byte("THIS-MUST-NEVER-BE-SERVED")
					Expect(os.WriteFile(strayPath, strayContents, 0o600)).To(Succeed())

					defer os.Remove(strayPath)

					req := initRequest(string(image), "/blobs/"+string(strayDigest), nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.BlobUnknown)
					Expect(body).NotTo(ContainSubstring(string(strayContents)))
				})
			})
		})

		Context("HEAD requests", func() {
			When("the blob exists", func() {
				It("returns 200 with Content-Length and Docker-Content-Digest, empty body", func() {
					req := initRequestWithMethod(
						http.MethodHead, string(image), "/blobs/"+string(configDigest), nil,
					)

					resp, body := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(configDigest.String()))
					Expect(resp.Header.Get("Content-Length")).To(Equal(
						strconv.Itoa(len(wantConfigBytes)),
					), "HEAD must advertise Content-Length matching the GET body")
					Expect(body).To(BeEmpty())
				})
			})

			When("the blob digest does not exist", func() {
				It("returns 404", func() {
					ghost := domain.Digest(
						"sha256:0000000000000000000000000000000000000000000000000000000000000002",
					)
					req := initRequestWithMethod(
						http.MethodHead, string(image), "/blobs/"+string(ghost), nil,
					)

					resp, _ := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Context("Pulling blobs via HTTPS in an unhealthy FS", Ordered, func() {
		var (
			re           *utils.RegistryEntry
			configDigest domain.Digest
		)

		BeforeEach(func() {
			re = &utils.RegistryEntry{
				Solution: "ipull-blob-solution",
				Version:  "v1.0.0",
				Image:    "docker.io/library/alpine",
				Tag:      "3.22.2",
			}
			suite.FetchImage(re)

			m := parseOnDiskManifest(re)
			Expect(m.Config).NotTo(BeNil())
			configDigest = m.Config.Digest
		})

		AfterEach(func() {
			suite.ClearImage(re)
		})

		When("the requested blob file is unreadable", func() {
			It("should soft-fail and return 404 BLOB_UNKNOWN", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				blobFile := filepath.Join(re.FullPath(suite.FsRoot), digestEncoded(configDigest))
				Expect(os.Chmod(blobFile, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(blobFile, utils.PermissionOK) }()

				req := initRequest(string(re.Image), "/blobs/"+string(configDigest), nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				checkErrorResponse(body, ocierrors.BlobUnknown)
			})
		})

		When("a manifest references a blob that is missing on disk", func() {
			It("should return 404 BLOB_UNKNOWN", func() {
				blobFile := filepath.Join(re.FullPath(suite.FsRoot), digestEncoded(configDigest))
				Expect(os.Remove(blobFile)).To(Succeed())

				req := initRequest(string(re.Image), "/blobs/"+string(configDigest), nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				checkErrorResponse(body, ocierrors.BlobUnknown)
			})
		})

		When("the manifest file is unreadable (authorization gate fails)", func() {
			It("should return 404 BLOB_UNKNOWN", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				manifestFile := filepath.Join(re.FullPath(suite.FsRoot), "manifest.json")
				Expect(os.Chmod(manifestFile, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(manifestFile, utils.PermissionOK) }()

				req := initRequest(string(re.Image), "/blobs/"+string(configDigest), nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				checkErrorResponse(body, ocierrors.BlobUnknown)
			})
		})
	})
})
