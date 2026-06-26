package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/ocilayout"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("Fetch Manifest From Digest", Ordered, func() {
	var (
		mockWalker *ocilayout.MockWalker
		fetcher    *digestmanifestfetcher.FileSystem
		image      domain.ImageName
	)

	BeforeEach(func() {
		image = "docker.io/library/alpine"
		mockWalker = ocilayout.NewMockWalker()

		var err error

		fetcher, err = digestmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Happy paths", func() {
		When("a reachable digest is in the first candidate layout", func() {
			It("returns the manifest output from that layout", func() {
				dgst := sha256Digest([]byte(validManifestJSON))

				layoutA := ocilayout.NewMockLayout()
				layoutA.ByDigest[dgst] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}
				mockWalker.Add(layoutA)

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal(domain.MediaTypeOCIImageManifest))
				Expect(out.ContentDigest).To(Equal(dgst))
				Expect(out.ManifestBytes).To(BeEquivalentTo([]byte(validManifestJSON)))
			})
		})

		When("the digest resolves to a multi-arch index", func() {
			It("returns the index output unchanged", func() {
				indexBytes := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json"}`)
				dgst := sha256Digest(indexBytes)

				layoutA := ocilayout.NewMockLayout()
				layoutA.ByDigest[dgst] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageIndex,
					ContentDigest: dgst,
					ManifestBytes: indexBytes,
				}
				mockWalker.Add(layoutA)

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

				layoutA := ocilayout.NewMockLayout()
				// ByDigest empty -- ReadManifestByDigest returns (nil, nil)

				layoutB := ocilayout.NewMockLayout()
				layoutB.ByDigest[dgst] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(dgst))
			})
		})
	})

	Context("Not found cases", func() {
		When("the digest is not reachable in any layout", func() {
			It("returns MANIFEST_UNKNOWN wrapping ErrManifestNotFound", func() {
				dgst := sha256Digest([]byte("nothing"))

				layoutA := ocilayout.NewMockLayout()
				// ByDigest empty -- not present
				mockWalker.Add(layoutA)

				_, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("the walker yields no layouts at all", func() {
			It("returns MANIFEST_UNKNOWN wrapping ErrManifestNotFound", func() {
				_, err := fetcher.FetchManifest(context.Background(), image, sha256Digest([]byte("nothing")))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(errors.Is(err, domain.ErrManifestNotFound)).To(BeTrue())
			})
		})

		When("an unsupported algorithm digest is requested (not in any layout's ByDigest)", func() {
			It("returns MANIFEST_UNKNOWN without special-casing the algorithm", func() {
				// sha512 digest not registered in any layout -- naturally returns not found.
				dgst := sha512Digest([]byte(validManifestJSON))

				layoutA := ocilayout.NewMockLayout()
				// ByDigest is empty, so dgst is not reachable
				mockWalker.Add(layoutA)

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
				mockWalker.WalkErr = errors.Wrap(domain.ErrRegistryInternal)

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

				layoutA := ocilayout.NewMockLayout()
				layoutA.DigestErr = errors.Wrap(domain.ErrRegistryInternal)

				layoutB := ocilayout.NewMockLayout()
				layoutB.ByDigest[dgst] = &domain.FetchManifestOutput{
					MediaType:     domain.MediaTypeOCIImageManifest,
					ContentDigest: dgst,
					ManifestBytes: []byte(validManifestJSON),
				}

				mockWalker.Add(layoutA)
				mockWalker.Add(layoutB)

				out, err := fetcher.FetchManifest(context.Background(), image, dgst)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.ContentDigest).To(Equal(dgst))
			})
		})
	})
})
