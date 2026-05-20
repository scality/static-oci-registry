package unit

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("List Tags", Ordered, func() {
	var (
		mockImageFinder *imagefinder.Mock
		tagLister       *taglister.FileSystem
		re              *utils.RegistryEntry
	)

	BeforeAll(func() {
		re = &utils.RegistryEntry{
			Solution: "list-tags-solution",
			Version:  "v1.0.0",
			Image:    "docker.io/library/alpine",
			Tag:      "3.22.2",
		}
		// mock the image finder with data from the fsRoot
		mockImageFinder = imagefinder.NewMock()
		mockImageFinder.AddImage(re.Solution, re.Version, re.Image)

		// finally, set up the tagLister
		var err error

		tagLister, err = taglister.NewFileSystem(suite.Logger, mockImageFinder, suite.FsRoot)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Listing tags in a healthy FS", func() {
		When("using an existing image", func() {
			It("should return the correct tags", func() {
				suite.FetchImage(re)

				tags, err := tagLister.ListTags(re.Image)
				Expect(err).NotTo(HaveOccurred())

				// validate contents of tags list
				Expect(tags).NotTo(BeNil())
				Expect(tags.Name).To(BeEquivalentTo(re.Image))
				Expect(tags.Tags).NotTo(BeEmpty())
				Expect(tags.Tags).To(HaveLen(1))
				Expect(tags.Tags).To(ContainElement(domain.Tag(re.Tag)))
			})
		})

		When("using a non existant image", func() {
			It("should return a not found error", func() {
				_, err := tagLister.ListTags("ghcr.io/nonexistent/image")
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrImageNotFound))
			})
		})
	})

	Context("Listing tags in an unhealthy fs", func() {
		Context("Listing tags when ImageFinder fails", func() {
			When("Listing tags for any image", func() {
				It("should return an internal error", func() {
					mockImageFinder.SetError(errors.Wrap(domain.ErrRegistryInternal))

					_, err := tagLister.ListTags(re.Image)
					Expect(err).To(HaveOccurred())
					utils.ValidateError(err)
					Expect(err).To(MatchError(domain.ErrRegistryInternal))
				})
			})
		})

		Context("Listing tags when reading image dir fails", func() {
			When("Listing tags for an existing image with bad permissions", func() {
				It("should return an internal error", func() {
					suite.FetchImage(re)
					os.Chmod(re.ImagePath(suite.FsRoot), utils.PermissionNoRead)

					_, err := tagLister.ListTags(re.Image)
					Expect(err).To(HaveOccurred())
					utils.ValidateError(err)
					Expect(err).To(MatchError(domain.ErrRegistryInternal))
				})
			})
		})
	})

	AfterEach(func() {
		suite.ClearImage(re)
		mockImageFinder.RemoveErrors()
	})
})
