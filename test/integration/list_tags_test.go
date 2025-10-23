package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/test/utils"
)

func checkListTagsOutput(body []byte, name string, tags []string) {
	output := &domain.ListTagsOutput{}

	Expect(json.Unmarshal(body, output)).To(Succeed())

	Expect(output.Name).To(BeEquivalentTo(name))

	// check len and elements - tags param should have unique values
	Expect(output.Tags).To(HaveLen(len(tags)))

	for _, tag := range tags {
		Expect(output.Tags).To(ContainElement(BeEquivalentTo(tag)))
	}
}

var _ = Describe("List Tags Integration", Ordered, func() {
	var client *http.Client

	BeforeEach(func() {
		client = &http.Client{
			Timeout: timeoutDurationInSeconds * time.Second,
		}
	})

	Context("Listing tags via HTTP in a healthy FS", Ordered, func() {
		solution := "list-tags-solution"
		var image domain.ImageName = "docker.io/library/alpine"
		tags := []string{"3.21.0", "3.22.0", "3.22.1", "3.22.2"}

		BeforeAll(func() {
			for _, tag := range tags {
				suite.FetchImage(&utils.RegistryEntry{
					Solution: solution,
					Version:  "v1.0.0",
					Image:    image,
					Tag:      tag,
				})
			}
		})

		When("using an existing image", func() {
			It("should return the correct tags", func() {
				req := initRequest(string(image), "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				checkListTagsOutput(body, string(image), tags)
			})
		})

		When("using valid n param", func() {
			DescribeTable("should return the correct number of tags", func(n int) {
				req := initRequest(string(image), "/tags/list", QueryParams{"n": strconv.Itoa(n)})

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				n = min(n, len(tags))
				checkListTagsOutput(body, string(image), tags[:n])
			},
				Entry("get no tags", 0),
				Entry("get partial tags", 2),
				Entry("get all tags", 500),
			)
		})

		When("using valid last param", func() {
			DescribeTable("should return the correct tags after last", func(last string) {
				req := initRequest(string(image), "/tags/list", QueryParams{"last": last})

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				n := slices.Index(tags, last)
				checkListTagsOutput(body, string(image), tags[n+1:])
			},
				Entry("get all tags", "3.21.0"),
				Entry("get partial tags", "3.22.0"),
				Entry("get no tags", "3.22.2"),
			)
		})

		// use both valid n and last params
		When("using valid n and last params", func() {
			DescribeTable("should return the correct tags after last with limit n",
				func(last string, n int) {
					req := initRequest(string(image), "/tags/list", QueryParams{"last": last})

					resp, body := execRequest(client, req)

					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					N := slices.Index(tags, last)
					targetTags := tags[N+1:]

					n = min(n, len(targetTags))
					targetTags = targetTags[:n]

					checkListTagsOutput(body, string(image), targetTags)
				},
				Entry("get all tags", "3.21.0", 4),
				Entry("get partial tags", "3.22.0", 2),
				Entry("get no tags", "3.22.2", 2),
			)
		})

		When("using an non-existing image", func() {
			It("should return a NAME_UNKNOWN error", func() {
				req := initRequest(string(image)+"a", "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

				checkErrorResponse(body, ocierrors.NameUnknown)
			})
		})

		When("using invalid n param", func() {
			DescribeTable("should return a UNSUPPORTED error", func(n string) {
				req := initRequest(string(image), "/tags/list", QueryParams{"n": n})

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

				checkErrorResponse(body, ocierrors.Unsupported)
			},
				// here we use string to allow non-integer values
				Entry("negative n", "-1"),
				Entry("vey big n", "1000000"),
				Entry("non integer n", "abc"),
			)
		})

		When("using invalid last param", func() {
			DescribeTable("should return a UNSUPPORTED error", func(last string) {
				req := initRequest(string(image), "/tags/list", QueryParams{"last": last})

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

				checkErrorResponse(body, ocierrors.Unsupported)
			},
				Entry("invalid tag", "==invalid"),
				Entry("non-existing tag", "3.333"),
			)
		})

		AfterAll(func() {
			for _, tag := range tags {
				suite.ClearImage(&utils.RegistryEntry{
					Solution: solution,
					Version:  "v1.0.0",
					Image:    image,
					Tag:      tag,
				})
			}
		})
	})

	Context("Listing tags via HTTP in an unhealthy FS", Ordered, func() {
		var re *utils.RegistryEntry

		BeforeAll(func() {
			re = &utils.RegistryEntry{
				Solution: "ilist-tags-solution",
				Version:  "v1.0.0",
				Image:    "docker.io/library/alpine",
				Tag:      "3.22.2",
			}
		})

		When("the image directory has bad permissions", func() {
			It("should return a 500 error", func() {
				suite.FetchImage(re)

				os.Chmod(re.ImagePath(suite.FsRoot), utils.PermissionNoRead)

				req := initRequest(string(re.Image), "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

				Expect(body).To(BeEmpty(), string(body))
			})
		})

		When("the image directory is not readable", func() {
			It("should return a 500 error", func() {
				suite.FetchImage(re)

				// copy the struct by dereferencing the pointer so we don't change the original
				rebad := *re
				rebad.Version = "v9.9.9"
				os.MkdirAll(rebad.ImagePath(suite.FsRoot), utils.PermissionNone)

				req := initRequest(string(re.Image), "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

				Expect(body).To(BeEmpty(), string(body))
			})
		})

		When("a solution directory is not readable", func() {
			It("should return a 500 error", func() {
				os.MkdirAll(suite.FsRoot+"/find-images-solution-baddir", utils.PermissionNoRead)

				req := initRequest(string(re.Image), "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

				Expect(body).To(BeEmpty(), string(body))

				os.Chmod(suite.FsRoot+"/find-images-solution-baddir", utils.PermissionOK)
				os.RemoveAll(suite.FsRoot + "/find-images-solution-baddir")
			})
		})

		When("the FS root is not readable", func() {
			It("should return a 500 error", func() {
				os.Chmod(suite.FsRoot, utils.PermissionNoRead)

				req := initRequest(string(re.Image), "/tags/list", nil)

				resp, body := execRequest(client, req)

				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

				Expect(body).To(BeEmpty(), string(body))
			})
		})

		AfterEach(func() {
			suite.ClearImage(re)
			os.Chmod(suite.FsRoot, utils.PermissionOK)
		})
	})
})
