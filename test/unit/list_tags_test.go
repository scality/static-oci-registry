package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/taglister"
	"github.com/scality/static-oci-registry/pkg/service/mocks"
	mock "github.com/stretchr/testify/mock"
)

var _ = Describe("List Tags", func() {
	var (
		mockWalker *mocks.MockLayoutWalker
		tagLister  *taglister.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = mocks.NewMockLayoutWalker(GinkgoT())

		var err error

		tagLister, err = taglister.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("a single layout yields several distinct tags", func() {
			It("returns all of them, sorted", func() {
				l := mocks.NewMockLayout(GinkgoT())
				l.EXPECT().Tags(mock.Anything).Return([]domain.Tag{"3.22.3", "3.22.2", "latest"}, nil)
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(l))

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
				l1 := mocks.NewMockLayout(GinkgoT())
				l1.EXPECT().Tags(mock.Anything).Return([]domain.Tag{"3.22.2"}, nil)

				l2 := mocks.NewMockLayout(GinkgoT())
				l2.EXPECT().Tags(mock.Anything).Return([]domain.Tag{"3.22.2"}, nil)

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(l1, l2))

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Tags).To(HaveLen(1))
				Expect(out.Tags).To(ContainElement(domain.Tag("3.22.2")))
			})
		})

		When("the walker yields no layouts", func() {
			It("returns an empty tag list without error", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq())

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.Name).To(Equal(image))
				Expect(out.Tags).To(BeEmpty())
			})
		})

		When("one layout's Tags() errors but others succeed", func() {
			It("skips the failing layout and returns the remaining tags", func() {
				l1 := mocks.NewMockLayout(GinkgoT())
				l1.EXPECT().Tags(mock.Anything).Return(nil, errors.New("index.json missing"))
				l1.EXPECT().Location().Return("sol/1.0.0/img").Maybe()

				l2 := mocks.NewMockLayout(GinkgoT())
				l2.EXPECT().Tags(mock.Anything).Return([]domain.Tag{"3.22.2", "latest"}, nil)

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(l1, l2))

				out, err := tagLister.ListTags(context.Background(), image)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.Tags).To(ConsistOf(domain.Tag("3.22.2"), domain.Tag("latest")))
			})
		})
	})

	Context("Error propagation", func() {
		When("the walker itself yields an error", func() {
			It("returns an error", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(errSeq(errors.New("imagefinder failed")))

				_, err := tagLister.ListTags(context.Background(), image)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
