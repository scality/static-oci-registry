package unit

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
	"github.com/scality/static-oci-registry/test/utils"
)

const fakeManifestJSON = `{"schemaVersion":2}`

// writeTagDir creates <fsRoot>/<sol>/<ver>/<image>/<tag>/manifest.json with dummy content.
// The walker only checks for presence + non-dirness of manifest.json, so the bytes are irrelevant.
func writeTagDir(root, sol, ver string, image domain.ImageName, tag string) {
	dir := filepath.Join(root, sol, ver, string(image), tag)
	Expect(os.MkdirAll(dir, utils.PermissionOK)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(fakeManifestJSON), 0o600)).To(Succeed())
}

func collectWalk(it func(yield func(domain.TagEntry, error) bool)) ([]domain.TagEntry, error) {
	var entries []domain.TagEntry

	for entry, err := range it {
		if err != nil {
			return entries, err
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

var _ = Describe("Walk Tags", Ordered, func() {
	var (
		mockImageFinder *imagefinder.Mock
		walker          *tagwalker.FileSystem
		image           domain.ImageName
	)

	BeforeAll(func() {
		image = "docker.io/library/alpine"

		mockImageFinder = imagefinder.NewMock()

		var err error

		walker, err = tagwalker.NewFileSystem(suite.Logger, mockImageFinder, openRoot(suite.FsRoot))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockImageFinder.RemoveErrors()
		// scrub everything inside fsRoot but leave the dir itself
		Expect(os.Chmod(suite.FsRoot, utils.PermissionOK)).To(Succeed())

		entries, err := os.ReadDir(suite.FsRoot)
		Expect(err).NotTo(HaveOccurred())

		for _, e := range entries {
			_ = os.Chmod(filepath.Join(suite.FsRoot, e.Name()), utils.PermissionOK)
			Expect(os.RemoveAll(filepath.Join(suite.FsRoot, e.Name()))).To(Succeed())
		}
		// reset the mock between tests
		mockImageFinder = imagefinder.NewMock()

		var err2 error

		walker, err2 = tagwalker.NewFileSystem(suite.Logger, mockImageFinder, openRoot(suite.FsRoot))
		Expect(err2).NotTo(HaveOccurred())
	})

	Context("Walking tags in a healthy FS", func() {
		When("the image has one valid tag in one solution-version", func() {
			It("yields exactly that tag entry", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Solution).To(Equal("sol-a"))
				Expect(entries[0].Version).To(Equal("v1.0.0"))
				Expect(entries[0].Name).To(Equal(image))
				Expect(entries[0].Tag).To(BeEquivalentTo("3.22.2"))
			})
		})

		When("the image has multiple valid tags in one solution-version", func() {
			It("yields all of them", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.3")
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "latest")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())

				tags := make([]domain.Tag, 0, len(entries))
				for _, e := range entries {
					tags = append(tags, e.Tag)
				}

				Expect(tags).To(ConsistOf(domain.Tag("3.22.2"), domain.Tag("3.22.3"), domain.Tag("latest")))
			})
		})

		When("the image lives in multiple solution-versions", func() {
			It("yields entries from every candidate", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				mockImageFinder.AddImage("sol-b", "v2.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				writeTagDir(suite.FsRoot, "sol-b", "v2.0.0", image, "latest")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(2))
			})
		})

		When("the same (image, tag) appears in two solution-versions", func() {
			It("yields both entries (no walker-level dedup)", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				mockImageFinder.AddImage("sol-b", "v2.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				writeTagDir(suite.FsRoot, "sol-b", "v2.0.0", image, "3.22.2")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(2))

				solutions := []string{entries[0].Solution, entries[1].Solution}
				Expect(solutions).To(ConsistOf("sol-a", "sol-b"))
			})
		})

		When("the caller breaks out of the range early", func() {
			It("stops the walk after the first entry", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "a")
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "b")
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "c")

				count := 0

				for _, err := range walker.WalkTags(context.Background(), image) {
					Expect(err).NotTo(HaveOccurred())

					count++

					break
				}

				Expect(count).To(Equal(1))
			})
		})
	})

	Context("Per-entry skip cases (walker logs and continues)", func() {
		When("a non-directory file sits in the image directory", func() {
			It("skips it and yields the valid sibling tag", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				// stray file alongside tag dirs
				strayFile := filepath.Join(suite.FsRoot, "sol-a", "v1.0.0", string(image), "README")
				Expect(os.WriteFile(strayFile, []byte("hi"), 0o600)).To(Succeed())

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Tag).To(BeEquivalentTo("3.22.2"))
			})
		})

		When("a tag directory has a name that fails domain.Tag.Validate", func() {
			It("skips it and yields the valid sibling tag", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				// leading dot is invalid per the OCI tag grammar
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, ".invalid-tag")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Tag).To(BeEquivalentTo("3.22.2"))
			})
		})

		When("a tag directory has no manifest.json", func() {
			It("skips it and yields the valid sibling tag", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				Expect(os.MkdirAll(
					filepath.Join(suite.FsRoot, "sol-a", "v1.0.0", string(image), "no-manifest"),
					utils.PermissionOK,
				)).To(Succeed())

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Tag).To(BeEquivalentTo("3.22.2"))
			})
		})

		When("manifest.json is itself a directory", func() {
			It("skips that tag and yields the valid sibling tag", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")
				badTagDir := filepath.Join(suite.FsRoot, "sol-a", "v1.0.0", string(image), "bad-tag")
				Expect(os.MkdirAll(filepath.Join(badTagDir, "manifest.json"), utils.PermissionOK)).To(Succeed())

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				Expect(entries[0].Tag).To(BeEquivalentTo("3.22.2"))
			})
		})
	})

	Context("Terminal-error cases", func() {
		When("ImageFinder returns an error", func() {
			It("yields the wrapped error and stops", func() {
				mockImageFinder.SetError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("the image directory is unreadable", func() {
			It("skips it and yields no entries (no error)", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")

				imageDir := filepath.Join(suite.FsRoot, "sol-a", "v1.0.0", string(image))
				Expect(os.Chmod(imageDir, utils.PermissionNoRead)).To(Succeed())

				defer func() { _ = os.Chmod(imageDir, utils.PermissionOK) }()

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(BeEmpty())
			})
		})
	})

	Context("ReadManifestBytes", func() {
		When("the manifest file exists", func() {
			It("returns its bytes verbatim", func() {
				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")

				entries, err := collectWalk(walker.WalkTags(context.Background(), image))
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))

				bytes, err := walker.ReadManifestBytes(entries[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(string(bytes)).To(Equal(fakeManifestJSON))
			})
		})

		When("the manifest file is missing", func() {
			It("returns a wrapped error", func() {
				entry := domain.TagEntry{
					SolutionVersion: domain.SolutionVersion{Solution: "ghost-sol", Version: "v0.0.0"},
					Name:            image,
					Tag:             domain.Tag("nope"),
				}

				_, err := walker.ReadManifestBytes(entry)
				Expect(err).To(HaveOccurred())
			})
		})

		When("the manifest file is unreadable", func() {
			It("returns a wrapped error", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				mockImageFinder.AddImage("sol-a", "v1.0.0", image)
				writeTagDir(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2")

				manifestFile := filepath.Join(
					suite.FsRoot, "sol-a", "v1.0.0", string(image), "3.22.2", "manifest.json",
				)
				Expect(os.Chmod(manifestFile, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(manifestFile, utils.PermissionOK) }()

				entry := domain.TagEntry{
					SolutionVersion: domain.SolutionVersion{Solution: "sol-a", Version: "v1.0.0"},
					Name:            image,
					Tag:             domain.Tag("3.22.2"),
				}
				_, err := walker.ReadManifestBytes(entry)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
