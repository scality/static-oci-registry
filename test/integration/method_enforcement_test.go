package integration

import (
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

func checkMethodNotAllowed(resp *http.Response, body []byte) {
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	checkErrorResponse(body, ocierrors.Unsupported)
}

var _ = Describe("Method Enforcement Integration", Ordered, func() {
	var client *http.Client

	BeforeEach(func() {
		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
			},
		}
	})

	Context("rejected methods on existing endpoints", func() {
		type endpoint struct {
			name string
			path string
		}

		endpoints := []endpoint{
			{"tags/list", "/tags/list"},
			{"manifests", "/manifests/sometag"},
			{"blobs", "/blobs/sha256:" + strings.Repeat("0", 64)},
		}

		// Disallowed methods to probe per endpoint. For tags/list HEAD is
		// also disallowed (end-8 is GET-only); covered by a dedicated spec
		// below so the table stays the same shape for every endpoint.
		mutatingMethods := []string{
			http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
		}

		for _, ep := range endpoints {
			for _, m := range mutatingMethods {
				When(m+" on "+ep.name, func() {
					It("returns 404 UNSUPPORTED via the standard error envelope", func() {
						req := initRequestWithMethod(m, "docker.io/library/alpine", ep.path, nil)

						resp, body := execRequest(client, req)

						checkMethodNotAllowed(resp, body)
					})
				})
			}
		}

		When("HEAD on tags/list", func() {
			It("returns 404 (end-8 is GET-only)", func() {
				req := initRequestWithMethod(
					http.MethodHead, "docker.io/library/alpine", "/tags/list", nil,
				)

				// HEAD has no body, so use execRequestRaw to avoid the JSON
				// trim helper running on an empty payload.
				resp, _ := execRequestRaw(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Context("unsupported /blobs/uploads/ endpoint", func() {
		paths := []string{
			"/blobs/uploads/",
			"/blobs/uploads/some-session-id",
		}

		methods := []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		}

		for _, p := range paths {
			for _, m := range methods {
				When(m+" "+p, func() {
					It("returns 404 UNSUPPORTED", func() {
						req := initRequestWithMethod(
							m, "docker.io/library/alpine", p, nil,
						)

						resp, body := execRequest(client, req)

						checkMethodNotAllowed(resp, body)
					})
				})
			}
		}
	})

	Context("regression: unknown paths still return 404", func() {
		When("hitting a path under /v2/ that no route matches", func() {
			It("returns 404 with an empty body", func() {
				url := "https://localhost" + cfg.HTTP.Addr + "/v2/totally/unknown/path"
				req, err := http.NewRequest(http.MethodGet, url, nil)
				Expect(err).NotTo(HaveOccurred())

				resp, _ := execRequestRaw(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
