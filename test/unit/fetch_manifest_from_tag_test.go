package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagmanifestfetcher"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("Fetch Manifest From Tag", Ordered, func() {
	var (
		mockWalker *ocilayout.MockWalker
		fetcher    *tagmanifestfetcher.FileSystem
		image      domain.ImageName
		tag        domain.Tag
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		tag = "3.22"
		mockWalker = ocilayout.NewMockWalker()

		var err error

		fetcher, err = tagmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	someDigest := domain.Digest("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	manifestBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`)

	Context("Happy paths", func() {
		When("a layout resolves the tag on the first candidate", func() {
			It("returns the output from that layout", func() {
				layoutA := ocilayout.NewMockLayout()
				layoutA.ByTag[tag] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}
				mockWalker.Add(layoutA)

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageManifest))
				Expect(out.ContentDigest).To(Equal(someDigest))
				Expect(out.ManifestBytes).To(BeEquivalentTo(manifestBytes))
			})
		})

		When("the first layout lacks the tag but the second has it", func() {
			It("returns the output from the second layout", func() {
				layoutA := ocilayout.NewMockLayout()
				// ByTag is empty -- ResolveTag returns (nil, nil)

				layoutB := ocilayout.NewMockLayout()
				layoutB.ByTag[tag] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(someDigest))
			})
		})

		When("a layout returns a multi-arch index for the tag", func() {
			It("passes the index output through unchanged", func() {
				indexDigest := domain.Digest("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
				indexBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json"}`)

				layoutA := ocilayout.NewMockLayout()
				layoutA.ByTag[tag] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageIndex,
					ContentDigest: indexDigest,
					ManifestBytes: indexBytes,
				}
				mockWalker.Add(layoutA)

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
				layoutA := ocilayout.NewMockLayout()
				// ByTag empty -- tag not present
				mockWalker.Add(layoutA)

				_, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("the walker yields no layouts at all", func() {
			It("returns MANIFEST_UNKNOWN OCI error wrapping ErrManifestNotFound", func() {
				// mockWalker has no layouts added
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
				mockWalker.WalkErr = errors.Wrap(domain.ErrRegistryInternal)

				_, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrRegistryInternal)).To(BeTrue())
			})
		})

		When("a layout's ResolveTag returns an error", func() {
			It("skips that layout and returns the result from the next one", func() {
				layoutA := ocilayout.NewMockLayout()
				layoutA.ResolveErr = errors.Wrap(domain.ErrRegistryInternal)

				layoutB := ocilayout.NewMockLayout()
				layoutB.ByTag[tag] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: someDigest,
					ManifestBytes: manifestBytes,
				}

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				out, err := fetcher.FetchManifest(context.Background(), image, tag)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(someDigest))
			})
		})
	})
})
