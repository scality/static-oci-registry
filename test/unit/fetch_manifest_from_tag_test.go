package unit

import (
	"context"
	"crypto/sha256"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagmanifestfetcher"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
	"github.com/scality/static-oci-registry/test/utils"
)

// validManifestJSON is a minimally-valid OCI image manifest per domain.Manifest.Validate().
// SchemaVersion=2, non-empty MediaType, Config with valid MediaType + digest.
const validManifestJSON = `{` +
	`"schemaVersion":2,` +
	`"mediaType":"application/vnd.oci.image.manifest.v1+json",` +
	`"config":{` +
	`"mediaType":"application/vnd.oci.image.config.v1+json",` +
	`"digest":"sha256:` +
	`0000000000000000000000000000000000000000000000000000000000000000",` +
	`"size":7023` +
	`},` +
	`"layers":[]` +
	`}`

func sha256Digest(bytes []byte) domain.Digest {
	h := sha256.Sum256(bytes)
	return domain.Digest(fmt.Sprintf("sha256:%x", h[:]))
}

var _ = Describe("Fetch Manifest From Tag", Ordered, func() {
	var (
		mockWalker *tagwalker.Mock
		fetcher    *tagmanifestfetcher.FileSystem
		image      domain.ImageName
	)

	BeforeAll(func() {
		image = "docker.io/library/alpine"

		mockWalker = tagwalker.NewMock()

		var err error

		fetcher, err = tagmanifestfetcher.NewFileSystem(suite.Logger, mockWalker)
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
		When("the walker yields a matching tag with a valid manifest", func() {
			It("returns the manifest with correct digest and media type", func() {
				bytes := []byte(validManifestJSON)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), bytes)

				out, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).NotTo(HaveOccurred())
				Expect(out).NotTo(BeNil())
				Expect(out.MediaType).To(Equal("application/vnd.oci.image.manifest.v1+json"))
				Expect(out.ContentDigest).To(Equal(sha256Digest(bytes)))
				Expect(out.ManifestBytes).To(BeEquivalentTo(bytes))
			})
		})

		When("the walker yields multiple tags including the target", func() {
			It("returns the matching tag's manifest", func() {
				targetBytes := []byte(validManifestJSON)
				otherBytes := []byte(`{"schemaVersion":2,"mediaType":"x","config":{"mediaType":"x","digest":"sha256:` +
					`1111111111111111111111111111111111111111111111111111111111111111","size":1}}`)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "other"), otherBytes)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), targetBytes)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "latest"), otherBytes)

				out, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).NotTo(HaveOccurred())
				Expect(out.ManifestBytes).To(BeEquivalentTo(targetBytes))
				Expect(out.ContentDigest).To(Equal(sha256Digest(targetBytes)))
			})
		})
	})

	Context("Not found cases", func() {
		When("the walker yields no entries", func() {
			It("returns ErrManifestNotFound", func() {
				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrManifestNotFound))
			})
		})

		When("the walker yields entries but none match the tag", func() {
			It("returns ErrManifestNotFound", func() {
				bytes := []byte(validManifestJSON)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "other"), bytes)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "latest"), bytes)

				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
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

				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("ReadManifestBytes fails on the matching tag", func() {
			It("returns a wrapped RegistryInternal error", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte(validManifestJSON))
				mockWalker.SetReadError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("the manifest bytes are not valid JSON", func() {
			It("returns a wrapped RegistryInternal error", func() {
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte("not json"))

				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})

		When("the manifest fails domain validation", func() {
			It("returns a wrapped RegistryInternal error", func() {
				// SchemaVersion 1 is invalid (must be 2)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte(`{"schemaVersion":1}`))

				_, err := fetcher.FetchManifest(context.Background(), image, domain.Tag("3.22.2"))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})
	})
})
