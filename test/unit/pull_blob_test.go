package unit

import (
	"context"
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/blobpuller"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("Pull Blob", Ordered, func() {
	var (
		mockWalker *ocilayout.MockWalker
		puller     *blobpuller.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = ocilayout.NewMockWalker()

		var err error

		puller, err = blobpuller.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("the blob is present in the first candidate layout", func() {
			It("returns a reader whose contents match the blob bytes", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := ocilayout.NewMockLayout()
				layoutA.Blobs[dgst] = blobContent
				mockWalker.Add(layoutA)

				rc, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(rc).NotTo(BeNil())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobContent))
			})
		})

		When("the first layout lacks the blob but the second has it", func() {
			It("returns the blob bytes from the second layout", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := ocilayout.NewMockLayout()
				// layoutA.Blobs is empty -- OpenBlob returns (nil, nil)

				layoutB := ocilayout.NewMockLayout()
				layoutB.Blobs[dgst] = blobContent

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				rc, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(rc).NotTo(BeNil())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobContent))
			})
		})

		When("a layout's OpenBlob returns a per-layout error but the next layout has the blob", func() {
			It("soft-fails past the erroring layout and returns bytes from the next", func() {
				blobContent := []byte("blob-content")
				dgst := sha256Digest(blobContent)

				layoutA := ocilayout.NewMockLayout()
				layoutA.BlobErr = errors.Wrap(domain.ErrRegistryInternal)

				layoutB := ocilayout.NewMockLayout()
				layoutB.Blobs[dgst] = blobContent

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				rc, err := puller.PullBlob(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(rc).NotTo(BeNil())

				defer rc.Close()

				got, err := io.ReadAll(rc)
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

				layoutA := ocilayout.NewMockLayout()
				// Blobs is empty -- every OpenBlob returns (nil, nil)
				mockWalker.Add(layoutA)

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

				// mockWalker has no layouts added

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
				mockWalker.WalkErr = errors.Wrap(domain.ErrRegistryInternal)

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
