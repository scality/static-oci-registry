package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
)

var _ = Describe("List Tags", Ordered, func() {
	var (
		mockWalker *ocilayout.MockWalker
		tagLister  *taglister.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = ocilayout.NewMockWalker()

		var err error

		tagLister, err = taglister.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("a single layout yields several distinct tags", func() {
			It("returns all of them, sorted", func() {
				l := ocilayout.NewMockLayout()
				l.TagList = []domain.Tag{"3.22.3", "3.22.2", "latest"}
				mockWalker.Add(l)

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Name).To(Equal(image))
				Expect(out.Tags).To(HaveLen(3))
				Expect(out.Tags).To(BeEquivalentTo([]domain.Tag{"3.22.2", "3.22.3", "latest"}))
			})
		})

		When("the same tag appears in multiple layouts", func() {
			It("deduplicates across layouts", func() {
				l1 := ocilayout.NewMockLayout()
				l1.TagList = []domain.Tag{"3.22.2"}
				mockWalker.Add(l1)

				l2 := ocilayout.NewMockLayout()
				l2.TagList = []domain.Tag{"3.22.2"}
				mockWalker.Add(l2)

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Tags).To(HaveLen(1))
				Expect(out.Tags).To(ContainElement(domain.Tag("3.22.2")))
			})
		})

		When("the walker yields no layouts", func() {
			It("returns an empty tag list without error", func() {
				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Name).To(Equal(image))
				Expect(out.Tags).To(BeEmpty())
			})
		})

		When("one layout's Tags() errors but others succeed", func() {
			It("skips the failing layout and returns the remaining tags", func() {
				l1 := ocilayout.NewMockLayout()
				l1.TagsErr = errors.New("index.json missing")
				mockWalker.Add(l1)

				l2 := ocilayout.NewMockLayout()
				l2.TagList = []domain.Tag{"3.22.2", "latest"}
				mockWalker.Add(l2)

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Tags).To(ConsistOf(domain.Tag("3.22.2"), domain.Tag("latest")))
			})
		})
	})

	Context("Error propagation", func() {
		When("the walker itself yields an error", func() {
			It("returns an error", func() {
				mockWalker.WalkErr = errors.New("imagefinder failed")

				_, err := tagLister.ListTags(context.Background(), image)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
