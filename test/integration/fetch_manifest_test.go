package integration

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/test/utils"
)

func sha256Digest(bytes []byte) domain.Digest {
	h := sha256.Sum256(bytes)

	return domain.Digest(fmt.Sprintf("sha256:%x", h[:]))
}

func checkManifestResponse(
	resp *http.Response,
	body []byte,
	wantBytes []byte,
	wantMediaType string,
	wantDigest domain.Digest,
) {
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(resp.Header.Get("Content-Type")).To(Equal(wantMediaType))
	Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(wantDigest.String()))
	Expect(body).To(BeEquivalentTo(wantBytes))
}

var _ = Describe("Fetch Manifest Integration", Ordered, func() {
	var client *http.Client

	BeforeEach(func() {
		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	Context("Fetching manifests via HTTPS in a healthy FS", Ordered, func() {
		solution := "fetch-manifest-solution"

		var (
			image      domain.ImageName = "docker.io/library/alpine"
			tag                         = "3.22.2"
			re         *utils.RegistryEntry
			wantBytes  []byte
			wantDigest domain.Digest
			mediaType  string
		)

		BeforeAll(func() {
			re = &utils.RegistryEntry{
				Solution: solution,
				Version:  "v1.0.0",
				Image:    image,
				Tag:      tag,
			}
			suite.BuildImage(re)

			// For a multi-arch image, GET /manifests/<tag> returns the index.
			// wantBytes/wantDigest/mediaType are sourced from the index.json entry.
			desc := onDiskTagDescriptor(re, tag)
			wantBytes = onDiskBlob(re, desc.Digest)
			wantDigest = desc.Digest
			mediaType = desc.MediaType
		})

		AfterAll(func() {
			suite.ClearImage(re)
		})

		Context("GET happy paths", func() {
			When("fetching by tag with URL-encoded slashes in the image name", func() {
				It("returns the manifest verbatim with correct headers", func() {
					req := initRequest(string(image), "/manifests/"+tag, nil)

					resp, body := execRequestRaw(client, req)

					checkManifestResponse(resp, body, wantBytes, mediaType, wantDigest)
				})
			})

			When("fetching by tag with unencoded slashes in the image name", func() {
				It("returns the manifest verbatim with correct headers", func() {
					url := "https://localhost" + cfg.HTTP.Addr + "/v2/" + string(image) + "/manifests/" + tag
					req, err := http.NewRequest(http.MethodGet, url, nil)
					Expect(err).NotTo(HaveOccurred())

					resp, body := execRequestRaw(client, req)

					checkManifestResponse(resp, body, wantBytes, mediaType, wantDigest)
				})
			})

			When("fetching by digest", func() {
				It("returns the manifest verbatim with the requested digest echoed back", func() {
					req := initRequest(string(image), "/manifests/"+string(wantDigest), nil)

					resp, body := execRequestRaw(client, req)

					checkManifestResponse(resp, body, wantBytes, mediaType, wantDigest)
				})
			})
			When("fetching by tag with ns query param (containerd-style)", func() {
				It("returns the manifest verbatim with correct headers", func() {
					// containerd splits "docker.io/library/alpine" into
					// ns=docker.io and image=library/alpine; the handler must rejoin them.
					req := initRequest("library/alpine", "/manifests/"+tag, QueryParams{"ns": "docker.io"})

					resp, body := execRequestRaw(client, req)

					checkManifestResponse(resp, body, wantBytes, mediaType, wantDigest)
				})
			})
		})

		Context("GET 404 error envelopes", func() {
			When("the image does not exist", func() {
				It("should return a NAME_UNKNOWN error", func() {
					req := initRequest(string(image)+"-ghost", "/manifests/"+tag, nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.NameUnknown)
				})
			})

			When("the tag does not exist for an existing image", func() {
				It("should return a MANIFEST_UNKNOWN error", func() {
					req := initRequest(string(image), "/manifests/nonexistent-tag", nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.ManifestUnknown)
				})
			})

			When("the digest does not match any manifest for an existing image", func() {
				It("should return a MANIFEST_UNKNOWN error", func() {
					wrongDigest := sha256Digest([]byte("not the real manifest"))
					req := initRequest(string(image), "/manifests/"+string(wrongDigest), nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.ManifestUnknown)
				})
			})

			When("the requested digest uses an unsupported algorithm", func() {
				It("should return a MANIFEST_UNKNOWN error", func() {
					// sha512: grammar-valid, unsupported by digestmanifestfetcher
					unsupported := "sha512:" +
						"00000000000000000000000000000000000000000000000000000000000000000000" +
						"00000000000000000000000000000000000000000000000000000000000000"
					req := initRequest(string(image), "/manifests/"+unsupported, nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.ManifestUnknown)
				})
			})
		})

		Context("GET malformed requests", func() {
			When("the image name is invalid", func() {
				It("should return a NAME_INVALID error", func() {
					// uppercase letters are not allowed by the OCI image-name grammar
					req := initRequest("Docker/Library/Alpine", "/manifests/"+tag, nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.NameInvalid)
				})
			})

			When("the reference is neither a valid tag nor a valid digest", func() {
				It("should return a MANIFEST_UNKNOWN error", func() {
					req := initRequest(string(image), "/manifests/==invalid", nil)

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					checkErrorResponse(body, ocierrors.ManifestUnknown)
				})
			})

			When("the reference contains an unencoded slash", func() {
				It("should fall through to a plain 404 (router does not match)", func() {
					// the URL pattern requires the reference segment to contain no '/',
					// so the router does not dispatch to the manifest handler at all.
					url := "https://localhost" + cfg.HTTP.Addr + "/v2/" + string(image) + "/manifests/foo/bar"
					req, err := http.NewRequest(http.MethodGet, url, nil)
					Expect(err).NotTo(HaveOccurred())

					resp, _ := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("HEAD requests (v1.0 spec end-3)", func() {
			When("the manifest exists, fetching by tag", func() {
				It("returns 200 with Docker-Content-Digest header and empty body", func() {
					req := initRequestWithMethod(http.MethodHead, string(image), "/manifests/"+tag, nil)

					resp, body := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(wantDigest.String()))
					Expect(resp.Header.Get("Content-Type")).To(Equal(mediaType))
					Expect(resp.Header.Get("Content-Length")).NotTo(BeEmpty(),
						"HEAD must advertise Content-Length matching the GET body")
					Expect(body).To(BeEmpty())
				})
			})

			When("the manifest exists, fetching by digest", func() {
				It("returns 200 with the requested digest echoed and empty body", func() {
					req := initRequestWithMethod(
						http.MethodHead, string(image), "/manifests/"+string(wantDigest), nil,
					)

					resp, body := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(resp.Header.Get("Docker-Content-Digest")).To(Equal(wantDigest.String()))
					Expect(body).To(BeEmpty())
				})
			})

			When("the tag does not exist", func() {
				It("returns 404", func() {
					req := initRequestWithMethod(
						http.MethodHead, string(image), "/manifests/nonexistent-tag", nil,
					)

					resp, _ := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			When("the digest does not exist", func() {
				It("returns 404", func() {
					wrongDigest := sha256Digest([]byte("not a real manifest"))
					req := initRequestWithMethod(
						http.MethodHead, string(image), "/manifests/"+string(wrongDigest), nil,
					)

					resp, _ := execRequestRaw(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Context("Fetching manifests via HTTPS in an unhealthy FS", Ordered, func() {
		var re *utils.RegistryEntry

		BeforeEach(func() {
			re = &utils.RegistryEntry{
				Solution: "ifetch-manifest-solution",
				Version:  "v1.0.0",
				Image:    "docker.io/library/alpine",
				Tag:      "3.22.2",
			}
			suite.BuildImage(re)
		})

		AfterEach(func() {
			suite.ClearImage(re)
		})

		When("the index.json is unreadable", func() {
			It("should soft-fail and return 404 MANIFEST_UNKNOWN", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				indexFile := filepath.Join(re.ImagePath(suite.FsRoot), "index.json")
				Expect(os.Chmod(indexFile, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(indexFile, utils.PermissionOK) }()

				req := initRequest(string(re.Image), "/manifests/"+re.Tag, nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(string(body)).To(ContainSubstring("MANIFEST_UNKNOWN"))
			})
		})

		When("the index.json contains invalid JSON", func() {
			It("should soft-fail and return 404 MANIFEST_UNKNOWN", func() {
				indexFile := filepath.Join(re.ImagePath(suite.FsRoot), "index.json")
				Expect(os.WriteFile(indexFile, []byte("not json at all"), 0o600)).To(Succeed())

				req := initRequest(string(re.Image), "/manifests/"+re.Tag, nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(string(body)).To(ContainSubstring("MANIFEST_UNKNOWN"))
			})
		})

		When("the index.json fails schema validation", func() {
			It("should soft-fail and return 404 MANIFEST_UNKNOWN", func() {
				indexFile := filepath.Join(re.ImagePath(suite.FsRoot), "index.json")
				// schemaVersion 1 + missing required mediaType is invalid
				Expect(os.WriteFile(indexFile, []byte(`{"schemaVersion":1}`), 0o600)).To(Succeed())

				req := initRequest(string(re.Image), "/manifests/"+re.Tag, nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(string(body)).To(ContainSubstring("MANIFEST_UNKNOWN"))
			})
		})
	})
})
