package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/service/mocks"
	"github.com/scality/static-oci-registry/test/utils"
	mock "github.com/stretchr/testify/mock"
)

var _ = Describe("Fetch Manifest From Tag", func() {
	var (
		mockWalker *mocks.MockLayoutWalker
		fetcher    *tagmanifestfetcher.FileSystem
		image      domain.ImageName
		tag        domain.Tag
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		tag = "3.22"
		mockWalker = mocks.NewMockLayoutWalker(GinkgoT())

		var err error

		fetcher, err = tagmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	someDigest := domain.Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	manifestBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`)

	Context("Happy paths", func() {
		When("a layout resolves the tag on the first candidate", func() {
			It("returns the output from that layout", func() {
				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ResolveTag(mock.Anything, tag).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}, nil)
				layoutA.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageManifest))
				Expect(out.ContentDigest).To(Equal(someDigest))
				Expect(out.ManifestBytes).To(BeEquivalentTo(manifestBytes))
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"}))
			})
		})

		When("the first layout lacks the tag but the second has it", func() {
			It("returns the output from the second layout", func() {
				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ResolveTag(mock.Anything, tag).Return(nil, nil)

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().ResolveTag(mock.Anything, tag).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}, nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(someDigest))
				Expect(out.SolutionVersion).To(Equal(domain.SolutionVersion{Solution: "sol", Version: "2.0.0"}))
			})
		})

		When("a layout returns a multi-arch index for the tag", func() {
			It("passes the index output through unchanged", func() {
				indexDigest := domain.Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
				indexBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json"}`)

				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ResolveTag(mock.Anything, tag).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageIndex,
					ContentDigest: indexDigest,
					ManifestBytes: indexBytes,
				}, nil)
				layoutA.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageIndex))
				Expect(out.ContentDigest).To(Equal(indexDigest))
				Expect(out.ManifestBytes).To(BeEquivalentTo(indexBytes))
			})
		})
	})

	Context("Not found cases", func() {
		When("no layout has the tag", func() {
			It("returns MANIFEST_UNKNOWN OCI error wrapping ErrManifestNotFound", func() {
				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ResolveTag(mock.Anything, tag).Return(nil, nil)
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA))

				_, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("the walker yields no layouts at all", func() {
			It("returns MANIFEST_UNKNOWN OCI error wrapping ErrManifestNotFound", func() {
				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq())

				_, err := fetcher.FetchManifest(context.Background(), image, tag)
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

				_, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrRegistryInternal)).To(BeTrue())
			})
		})

		When("a layout's ResolveTag returns an error", func() {
			It("skips that layout and returns the result from the next one", func() {
				layoutA := mocks.NewMockLayout(GinkgoT())
				layoutA.EXPECT().ResolveTag(mock.Anything, tag).Return(nil, errors.Wrap(domain.ErrRegistryInternal))
				layoutA.EXPECT().Location().Return("sol/1.0.0/img").Maybe()

				layoutB := mocks.NewMockLayout(GinkgoT())
				layoutB.EXPECT().ResolveTag(mock.Anything, tag).Return(&domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}, nil)
				layoutB.EXPECT().SolutionVersion().Return(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})

				mockWalker.EXPECT().WalkLayouts(mock.Anything, image).Return(layoutSeq(layoutA, layoutB))

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(someDigest))
			})
		})
	})
})
