package unit

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/digestmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("Fetch Manifest From Digest", Ordered, func() {
	var (
		mockWalker *tagwalker.Mock
		fetcher    *digestmanifestfetcher.FileSystem
		image      domain.ImageName
	)

	BeforeAll(func() {
		image = "docker.io/library/alpine"

		mockWalker = tagwalker.NewMock()

		var err error

		fetcher, err = digestmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
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
		When("a walker entry's manifest hashes to the requested digest", func() {
			It("returns the manifest with the requested digest echoed back", func() {
				bytes := []byte(validManifestJSON)
				wantDigest := sha256Digest(bytes)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), bytes)

				out, err := fetcher.FetchManifest(context.Background(), image, wantDigest)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal("application/vnd.oci.image.manifest.v1+json"))
				Expect(out.ContentDigest).To(Equal(wantDigest))
				Expect(out.ManifestBytes).To(BeEquivalentTo(bytes))
			})
		})

		When("only one of several entries matches the digest", func() {
			It("returns that entry's manifest", func() {
				targetBytes := []byte(validManifestJSON)
				otherBytes := []byte(`{` +
					`"schemaVersion":2,` +
					`"mediaType":"application/vnd.oci.image.manifest.v1+json",` +
					`"config":{"mediaType":"x","digest":"sha256:` +
					`1111111111111111111111111111111111111111111111111111111111111111","size":1},` +
					`"layers":[]` +
					`}`)
				wantDigest := sha256Digest(targetBytes)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "other"), otherBytes)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), targetBytes)

				out, err := fetcher.FetchManifest(context.Background(), image, wantDigest)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.ManifestBytes).To(BeEquivalentTo(targetBytes))
			})
		})

		When("a walker entry's manifest hashes to the requested sha512 digest", func() {
			It("returns the manifest with the requested digest echoed back", func() {
				bytes := []byte(validManifestJSON)
				wantDigest := sha512Digest(bytes)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), bytes)

				out, err := fetcher.FetchManifest(context.Background(), image, wantDigest)
				Expect(err).NotTo(HaveOccurred())
				Expect(out.ContentDigest).To(Equal(wantDigest))
				Expect(out.ManifestBytes).To(Equal(bytes))
			})
		})
	})

	Context("Not found cases", func() {
		When("the walker yields no entries", func() {
			It("returns ErrManifestNotFound", func() {
				_, err := fetcher.FetchManifest(context.Background(), image, sha256Digest([]byte("nothing")))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})

		When("no entry's manifest matches the requested digest", func() {
			It("returns ErrManifestNotFound", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte(validManifestJSON))

				wrongDigest := sha256Digest([]byte("something completely different"))

				_, err := fetcher.FetchManifest(context.Background(), image, wrongDigest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})

		When("the requested digest uses an unsupported algorithm", func() {
			It("returns ErrManifestNotFound without walking", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte(validManifestJSON))
				// sha384 is grammar-valid but unsupported by the fetcher.
				unsupported := domain.Digest("sha384:" +
					"000000000000000000000000000000000000000000000000" +
					"000000000000000000000000000000000000000000000000")

				_, err := fetcher.FetchManifest(context.Background(), image, unsupported)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})
	})

	Context("Error propagation", func() {
		When("the walker yields an error", func() {
			It("propagates the wrapped error", func() {
				mockWalker.SetWalkError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := fetcher.FetchManifest(context.Background(), image,
					sha256Digest([]byte(validManifestJSON)))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("ReadManifestBytes fails", func() {
			It("skips the entry and returns ManifestNotFound", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte(validManifestJSON))
				mockWalker.SetReadError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := fetcher.FetchManifest(context.Background(), image,
					sha256Digest([]byte(validManifestJSON)))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})

		When("the manifest bytes are not valid JSON", func() {
			It("skips the entry and returns ManifestNotFound", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte("not json"))

				_, err := fetcher.FetchManifest(context.Background(), image,
					sha256Digest([]byte("not json")))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})

		When("the manifest fails domain validation", func() {
			It("skips the entry and returns ManifestNotFound", func() {
				badBytes := []byte(`{"schemaVersion":1}`)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), badBytes)

				_, err := fetcher.FetchManifest(context.Background(), image, sha256Digest(badBytes))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})
	})
})
