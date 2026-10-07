package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/service/mocks"
	"github.com/scality/static-oci-registry/test/utils"
	mock "github.com/stretchr/testify/mock"
)

var _ = Describe("Fetch Manifest From Digest", func() {
	var (
		mockWalker *mocks.MockLayoutWalker
		fetcher    *digestmanifestfetcher.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = mocks.NewMockLayoutWalker(GinkgoT())

		var err error

		fetcher, err = digestmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("a reachable digest is in the first candidate layout", func() {
			It("returns the manifest output from that layout", func() {
				dgst := sha256Digest([]byte(validManifestJSON))

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}, nil)
				layoutA.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageManifest))
				Expect(out.ContentDigest).To(Equal(dgst))
				Expect(out.ManifestBytes).To(BeEquivalentTo([]byte(validManifestJSON)))
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"}))
			})
		})

		When("the digest resolves to a multi-arch index", func() {
			It("returns the index output unchanged", func() {
				indexBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json"}`)
				dgst := sha256Digest(indexBytes)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageIndex,
					ContentDigest: dgst,
					ManifestBytes: indexBytes,
				}, nil)
				layoutA.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageIndex))
				Expect(out.ContentDigest).To(Equal(dgst))
				Expect(out.ManifestBytes).To(BeEquivalentTo(indexBytes))
			})
		})

		When("the first layout lacks the digest but the second has it", func() {
			It("returns the output from the second layout", func() {
				dgst := sha256Digest([]byte(validManifestJSON))

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(nil, nil)

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}, nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(dgst))
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"}))
			})
		})
	})

	Context("Not found cases", func() {
		When("the digest is not reachable in any layout", func() {
			It("returns MANIFEST_UNKNOWN wrapping ErrManifestNotFound", func() {
				dgst := sha256Digest([]byte("nothing"))

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(nil, nil)
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				_, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("the walker yields no layouts at all", func() {
			It("returns MANIFEST_UNKNOWN wrapping ErrManifestNotFound", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq())

				_, err := fetcher.FetchManifest(context.Background(), image, sha256Digest([]byte("nothing")))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("an unsupported algorithm digest is requested (not in any layout's index)", func() {
			It("returns MANIFEST_UNKNOWN without special-casing the algorithm", func() {
				// sha512 digest not registered in any layout -- naturally returns not found.
				dgst := sha512Digest([]byte(validManifestJSON))

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(nil, nil)
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				_, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})
	})

	Context("Error propagation", func() {
		When("the walker itself yields an error", func() {
			It("propagates the wrapped error and aborts", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(errSeq(errors.Wrap(domain.ErrRegistryInternal)))

				_, err := fetcher.FetchManifest(context.Background(), image,
					sha256Digest([]byte(validManifestJSON)))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrRegistryInternal)).To(BeTrue())
			})
		})

		When("a layout's ReadManifestByDigest returns an error", func() {
			It("skips that layout and returns the result from the next one", func() {
				dgst := sha256Digest([]byte(validManifestJSON))

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(nil, errors.Wrap(domain.ErrRegistryInternal))
				layoutA.EXPECT().Location().Return("sol/1.0.0/img").Maybe()

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().ReadManifestByDigest(mock.Anything, dgst).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}, nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(dgst))
			})
		})
	})
})
