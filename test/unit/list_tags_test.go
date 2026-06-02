package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("List Tags", Ordered, func() {
	var (
		mockWalker *tagwalker.Mock
		tagLister  *taglister.FileSystem
		image      domain.ImageName
	)

	BeforeAll(func() {
		image = "docker.io/library/alpine"

		mockWalker = tagwalker.NewMock()

		var err error

		tagLister, err = taglister.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockWalker.Reset()
	})

	entry := func(sol, ver, tag string) domain.TagEntry {
		return domain.TagEntry{
			SolutionVersion: domain.SolutionVersion{Solution: sol, Version: ver},
			Name:            image,
			Tag:             domain.Tag(tag),
		}
	}

	Context("Happy paths", func() {
		When("the walker yields several distinct tags", func() {
			It("returns all of them, sorted", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.3"), nil)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), nil)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "latest"), nil)

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Name).To(Equal(image))
				Expect(out.Tags).To(HaveLen(3))

				Expect(out.Tags).To(BeEquivalentTo([]domain.Tag{"3.22.2", "3.22.3", "latest"}))
			})
		})

		When("the same tag is yielded from multiple solution-versions", func() {
			It("deduplicates", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), nil)
				mockWalker.AddEntry(entry("sol-b", "v2.0.0", "3.22.2"), nil)

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Tags).To(HaveLen(1))
				Expect(out.Tags).To(ContainElement(domain.Tag("3.22.2")))
			})
		})

		When("the walker yields no entries", func() {
			It("returns an empty tag list without error", func() {
				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Name).To(Equal(image))
				Expect(out.Tags).To(BeEmpty())
			})
		})
	})

	Context("Error propagation", func() {
		When("the walker yields an error", func() {
			It("wraps it and preserves the sentinel", func() {
				mockWalker.SetWalkError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := tagLister.ListTags(context.Background(), image)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("the walker yields an ImageNotFound error", func() {
			It("propagates the sentinel", func() {
				mockWalker.SetWalkError(errors.Wrap(
					domain.ErrImageNotFound,
					errors.WithDetail("image not found in filesystem registry"),
				))

				_, err := tagLister.ListTags(context.Background(), image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(domain.ErrImageNotFound))
			})
		})
	})
})
