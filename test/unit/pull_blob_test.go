package unit

import (
	"context"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/blobpuller"
	"github.com/scality/static-oci-registry/pkg/service/mocks"
	"github.com/scality/static-oci-registry/test/utils"
	mock "github.com/stretchr/testify/mock"
)

var _ = Describe("Pull Blob", func() {
	var (
		mockWalker *mocks.MockLayoutWalker
		puller     *blobpuller.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = mocks.NewMockLayoutWalker(GinkgoT())

		var err error

		puller, err = blobpuller.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("the blob is present in the first candidate layout", func() {
			It("returns a reader whose contents match the blob bytes", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().OpenBlob(mock.Anything, dgst).Return(blobReader(blobContent), nil)
				layoutA.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				out, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"}))

				defer out.Body.Close()

				got, err := io.ReadAll(out.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobContent))
			})
		})

		When("the first layout lacks the blob but the second has it", func() {
			It("returns the blob bytes from the second layout", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().OpenBlob(mock.Anything, dgst).Return(nil, nil)

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().OpenBlob(mock.Anything, dgst).Return(blobReader(blobContent), nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"}))

				defer out.Body.Close()

				got, err := io.ReadAll(out.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobContent))
			})
		})

		When("a layout's OpenBlob returns a per-layout error but the next layout has the blob", func() {
			It("soft-fails past the erroring layout and returns bytes from the next", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().OpenBlob(mock.Anything, dgst).Return(nil, errors.Wrap(domain.ErrRegistryInternal))
				layoutA.EXPECT().Location().Return("sol/1.0.0/img").Maybe()

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().OpenBlob(mock.Anything, dgst).Return(blobReader(blobContent), nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())

				defer out.Body.Close()

				got, err := io.ReadAll(out.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobContent))
			})
		})
	})

	Context("Miss paths", func() {
		When("no layout contains the requested digest", func() {
			It("returns ErrBlobNotFound with BLOB_UNKNOWN", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().OpenBlob(mock.Anything, dgst).Return(nil, nil)
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				_, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrBlobNotFound)).To(BeTrue())
			})
		})

		When("the walker yields no layouts at all", func() {
			It("returns ErrBlobNotFound with BLOB_UNKNOWN", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq())

				_, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrBlobNotFound)).To(BeTrue())
			})
		})
	})

	Context("Terminal errors", func() {
		When("the walker yields an error", func() {
			It("propagates the wrapped error", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(errSeq(errors.Wrap(domain.ErrRegistryInternal)))

				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				_, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrRegistryInternal)).To(BeTrue())
			})
		})
	})
})
