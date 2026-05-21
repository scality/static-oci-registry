package unit

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/test/utils"

	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
)

var _ = Describe("Find Images", Ordered, func() {
	var (
		imageFinder *imagefinder.FileSystem
		re          *utils.RegistryEntry
	)

	BeforeAll(func() {
		re = &utils.RegistryEntry{
			Solution: "find-images-solution",
			Version:  "v1.0.0",
			Image:    "docker.io/library/alpine",
			Tag:      "3.22.2",
		}

		var err error

		imageFinder, err = imagefinder.NewFileSystem(suite.Logger, suite.FsRoot)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Finding images in a healthy FS", func() {
		When("using an existing image", func() {
			It("should return the correct solution versions", func() {
				suite.FetchImage(re)

				found, err := imageFinder.FindImage(re.Image)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).NotTo(BeEmpty())
			})
		})

		When("using a non existant image", func() {
			It("should return a not found error", func() {
				suite.FetchImage(re)

				_, err := imageFinder.FindImage("ghcr.io/nonexistent/image")

				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrImageNotFound))
			})
		})
	})

	Context("Finding images in a corrupted FS", func() {
		When("an image directory is not readable", func() {
			It("should return a registry internal error", func() {
				suite.FetchImage(re)

				// copy the struct by dereferencing the pointer so we don't change the original
				rebad := *re
				rebad.Version = "v9.9.9"
				os.MkdirAll(rebad.ImagePath(suite.FsRoot), utils.PermissionNone)

				_, err := imageFinder.FindImage(re.Image)

				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("a solution directory is not readable", func() {
			It("should return a registry internal error", func() {
				os.MkdirAll(suite.FsRoot+"/find-images-solution-baddir", utils.PermissionNoRead)

				_, err := imageFinder.FindImage(re.Image)

				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))

				os.Chmod(suite.FsRoot+"/find-images-solution-baddir", utils.PermissionOK)
				os.RemoveAll(suite.FsRoot + "/find-images-solution-baddir")
			})
		})

		When("the FS root is not readable", func() {
			It("should return a registry internal error", func() {
				os.Chmod(suite.FsRoot, utils.PermissionNoRead)

				_, err := imageFinder.FindImage(re.Image)

				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})
	})

	AfterEach(func() {
		suite.ClearImage(re)
		os.Chmod(suite.FsRoot, utils.PermissionOK)
	})
})
