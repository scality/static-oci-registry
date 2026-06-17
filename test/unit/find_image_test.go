package unit

import (
	"context"
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

		imageFinder, err = imagefinder.NewFileSystem(suite.Logger, openRoot(suite.FsRoot))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Finding images in a healthy FS", func() {
		When("using an existing image", func() {
			It("should return the correct solution versions", func() {
				suite.FetchImage(re)

				found, err := imageFinder.FindImage(context.Background(), re.Image)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).NotTo(BeEmpty())
			})
		})

		When("using a non existant image", func() {
			It("should return a not found error", func() {
				suite.FetchImage(re)

				_, err := imageFinder.FindImage(context.Background(), "ghcr.io/nonexistent/image")

				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrImageNotFound))
			})
		})
	})

	Context("Finding images in a corrupted FS", func() {
		When("an image directory is not readable", func() {
			It("should skip the bad candidate and still return the healthy ones", func() {
				suite.FetchImage(re)

				// copy the struct by dereferencing the pointer so we don't change the original
				rebad := *re
				rebad.Version = "v9.9.9"
				os.MkdirAll(rebad.ImagePath(suite.FsRoot), utils.PermissionNone)

				found, err := imageFinder.FindImage(context.Background(), re.Image)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).NotTo(BeEmpty())
			})
		})

		When("a solution directory is not readable", func() {
			It("should return a registry internal error", func() {
				os.MkdirAll(suite.FsRoot+"/find-images-solution-baddir", utils.PermissionNoRead)

				_, err := imageFinder.FindImage(context.Background(), re.Image)

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

				_, err := imageFinder.FindImage(context.Background(), re.Image)

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

var _ = Describe("Find Images ordering (round-robin, newest version first)", Ordered, func() {
	// Layout under suite.FsRoot:
	//   rr-sol-a/{v1.0.0,v2.0.0,v3.0.0}/<image>/
	//   rr-sol-b/v1.0.0/<image>/
	//   rr-sol-c/{v1.0.0,v2.0.0}/<image>/
	//
	// Expected ordering returned by FindImage:
	//   (a:v3.0.0) (b:v1.0.0) (c:v2.0.0)  -- highest of each solution, round-robin
	//   (a:v2.0.0)             (c:v1.0.0) -- then next-highest
	//   (a:v1.0.0)                        -- then last
	const image domain.ImageName = "docker.io/round-robin/test-image"

	layout := map[string][]string{
		"rr-sol-a": {"v1.0.0", "v2.0.0", "v3.0.0"},
		"rr-sol-b": {"v1.0.0"},
		"rr-sol-c": {"v1.0.0", "v2.0.0"},
	}

	var imageFinder *imagefinder.FileSystem

	BeforeAll(func() {
		for solution, versions := range layout {
			for _, version := range versions {
				path := suite.FsRoot + "/" + solution + "/" + version + "/" + string(image)
				Expect(os.MkdirAll(path, utils.PermissionOK)).To(Succeed())
			}
		}

		var err error

		imageFinder, err = imagefinder.NewFileSystem(suite.Logger, openRoot(suite.FsRoot))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		for solution := range layout {
			Expect(os.RemoveAll(suite.FsRoot + "/" + solution)).To(Succeed())
		}
	})

	It("interleaves solutions and emits the newest version of each first", func() {
		found, err := imageFinder.FindImage(context.Background(), image)
		Expect(err).NotTo(HaveOccurred())

		// Exact ordering: highest version of each solution, then next-highest, etc.
		Expect(found).To(Equal([]domain.SolutionVersion{
			{Solution: "rr-sol-a", Version: "v3.0.0"},
			{Solution: "rr-sol-b", Version: "v1.0.0"},
			{Solution: "rr-sol-c", Version: "v2.0.0"},
			{Solution: "rr-sol-a", Version: "v2.0.0"},
			{Solution: "rr-sol-c", Version: "v1.0.0"},
			{Solution: "rr-sol-a", Version: "v1.0.0"},
		}))
	})
})
