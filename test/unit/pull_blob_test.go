package unit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/blobpuller"
	"github.com/scality/static-oci-registry/pkg/infrastructure/tagwalker"
	"github.com/scality/static-oci-registry/test/utils"
)

// blobBody is the bytes written to disk in happy-path tests. It is small on
// purpose; only identity (digest match) matters, not size.
var blobBody = []byte("test blob payload")

// writeBlobFile creates <fsRoot>/<sol>/<ver>/<image>/<tag>/<encoded> with the
// given body. The puller's on-disk layout mirrors tagwalker.manifestPath so
// the path computation is identical.
//
// future tests that exercise multiple solutions.
//
//nolint:unparam // sol is parameterised for parity with writeTagDir and for
func writeBlobFile(
	root, sol, ver string, image domain.ImageName, tag, encoded string, body []byte,
) {
	dir := filepath.Join(root, sol, ver, string(image), tag)
	Expect(os.MkdirAll(dir, utils.PermissionOK)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dir, encoded), body, 0o600)).To(Succeed())
}

// encoded strips the algorithm prefix from a digest. Helper used to build paths
// on disk. The puller calls digest.Encoded() internally; we mirror that here.
func encoded(d domain.Digest) string {
	parts := strings.SplitN(string(d), ":", 2)
	Expect(parts).To(HaveLen(2), "digest must be in <algo>:<hex> form")

	return parts[1]
}

// manifestWith builds a minimally-valid manifest JSON that references the
// given digests. Any of config/layers/subject may be empty: an empty
// configDigest is replaced with a placeholder (Manifest.Validate requires a
// non-nil Config with a valid digest), and an empty subjectDigest omits the
// Subject field entirely.
func manifestWith(configDigest domain.Digest, layerDigests []domain.Digest, subjectDigest domain.Digest) []byte {
	if configDigest == "" {
		configDigest = domain.Digest("sha256:" + strings.Repeat("0", 64))
	}

	layers := make([]domain.ManifestDescriptor, len(layerDigests))
	for i, ld := range layerDigests {
		layers[i] = domain.ManifestDescriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    ld,
			Size:      1,
		}
	}

	m := domain.Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: &domain.ManifestDescriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      1,
		},
		Layers: layers,
	}

	if subjectDigest != "" {
		m.Subject = &domain.ManifestDescriptor{
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Digest:    subjectDigest,
			Size:      1,
		}
	}

	bytes, err := json.Marshal(m)
	// Marshal failure here would be a programming error in the test fixture.
	Expect(err).NotTo(HaveOccurred())

	return bytes
}

var _ = Describe("Pull Blob", Ordered, func() {
	var (
		mockWalker *tagwalker.Mock
		puller     *blobpuller.FileSystem
		image      domain.ImageName
	)

	BeforeAll(func() {
		image = "docker.io/library/alpine"

		mockWalker = tagwalker.NewMock()

		var err error

		puller, err = blobpuller.NewFileSystem(suite.Logger, mockWalker, suite.FsRoot)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockWalker.Reset()
		// scrub everything inside fsRoot but leave the dir itself
		Expect(os.Chmod(suite.FsRoot, utils.PermissionOK)).To(Succeed())

		entries, err := os.ReadDir(suite.FsRoot)
		Expect(err).NotTo(HaveOccurred())

		for _, e := range entries {
			_ = os.Chmod(filepath.Join(suite.FsRoot, e.Name()), utils.PermissionOK)
			Expect(os.RemoveAll(filepath.Join(suite.FsRoot, e.Name()))).To(Succeed())
		}
	})

	entry := func(sol, ver, tag string) domain.TagEntry {
		return domain.TagEntry{
			SolutionVersion: domain.SolutionVersion{Solution: sol, Version: ver},
			Name:            image,
			Tag:             domain.Tag(tag),
		}
	}

	Context("NewFileSystem", func() {
		When("fsRoot does not exist", func() {
			It("returns a wrapped error", func() {
				missing := filepath.Join(suite.FsRoot, "does-not-exist")

				_, err := blobpuller.NewFileSystem(suite.Logger, mockWalker, missing)
				Expect(err).To(HaveOccurred())
			})
		})

		When("fsRoot is a regular file", func() {
			It("returns an error stating the path is not a directory", func() {
				file := filepath.Join(suite.FsRoot, "not-a-dir")
				Expect(os.WriteFile(file, []byte("x"), 0o600)).To(Succeed())

				_, err := blobpuller.NewFileSystem(suite.Logger, mockWalker, file)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not a directory"))
			})
		})

		When("fsRoot is a valid directory", func() {
			It("returns a usable puller", func() {
				p, err := blobpuller.NewFileSystem(suite.Logger, mockWalker, suite.FsRoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(p).NotTo(BeNil())
			})
		})
	})

	Context("Happy paths", func() {
		When("the requested digest matches the manifest config", func() {
			It("returns a reader for the blob bytes", func() {
				digest := sha256Digest(blobBody)
				mf := manifestWith(digest, nil, "")

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).NotTo(HaveOccurred())
				Expect(rc).NotTo(BeNil())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobBody))
			})
		})

		When("the requested digest matches a manifest layer", func() {
			It("returns a reader for the blob bytes", func() {
				layerDigest := sha256Digest(blobBody)
				otherLayer := domain.Digest("sha256:" + strings.Repeat("a", 64))
				mf := manifestWith("", []domain.Digest{otherLayer, layerDigest}, "")

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2",
					encoded(layerDigest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, layerDigest)
				Expect(err).NotTo(HaveOccurred())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobBody))
			})
		})

		When("the requested digest matches the manifest subject", func() {
			It("returns a reader for the blob bytes", func() {
				subjectDigest := sha256Digest(blobBody)
				mf := manifestWith("", nil, subjectDigest)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2",
					encoded(subjectDigest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, subjectDigest)
				Expect(err).NotTo(HaveOccurred())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobBody))
			})
		})

		When("the blob lives in the second of two candidate tag entries", func() {
			It("returns the blob from the second entry", func() {
				digest := sha256Digest(blobBody)
				otherDigest := domain.Digest("sha256:" + strings.Repeat("b", 64))

				// first entry's manifest references a different blob and has no file on disk
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(otherDigest, nil, ""))

				// second entry references the target digest and has it on disk
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.3"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.3", encoded(digest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).NotTo(HaveOccurred())

				defer rc.Close()

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobBody))
			})
		})

		When("a caller seeks on the returned reader", func() {
			It("supports Seek (http.ServeContent relies on this)", func() {
				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).NotTo(HaveOccurred())

				defer rc.Close()

				end, err := rc.Seek(0, io.SeekEnd)
				Expect(err).NotTo(HaveOccurred())
				Expect(end).To(Equal(int64(len(blobBody))))

				_, err = rc.Seek(0, io.SeekStart)
				Expect(err).NotTo(HaveOccurred())

				got, err := io.ReadAll(rc)
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(blobBody))
			})
		})

		When("the caller closes the returned reader", func() {
			It("succeeds once and a second Close returns an error", func() {
				// Ownership of Close belongs to the caller; the puller must
				// not pre-close or double-close the file handle.
				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				rc, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).NotTo(HaveOccurred())

				Expect(rc.Close()).To(Succeed())
				Expect(rc.Close()).To(HaveOccurred())
			})
		})
	})

	Context("Security: stray files are not servable", func() {
		When("a file exists in the tag dir but the manifest does not reference it", func() {
			It("returns ErrBlobNotFound", func() {
				// Client requests a digest whose hex collides with a stray file
				// in the tag directory (e.g. a "version" file or similar).
				// The on-disk file exists, but the manifest references a
				// completely different digest, so serving is refused.
				strayBody := []byte("not-in-manifest")
				strayDigest := sha256Digest(strayBody)

				referencedDigest := domain.Digest("sha256:" + strings.Repeat("c", 64))
				mf := manifestWith(referencedDigest, nil, "")

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2",
					encoded(strayDigest), strayBody)

				_, err := puller.PullBlob(context.Background(), image, strayDigest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})
	})

	Context("Security: algorithm matching", func() {
		When("client requests sha512:<hex> but manifest only references sha256:<same-hex>", func() {
			It("returns ErrBlobNotFound (no cross-algorithm match)", func() {
				// Construct a sha512 digest whose hex differs from the sha256
				// digest of the same body, but is grammar-valid. We force a
				// collision in the *file name* (encoded hex) by computing the
				// sha256 digest of the body, then crafting a sha512 digest
				// whose encoded portion matches the existing file path.
				sha256Dgst := sha256Digest(blobBody)
				// 64-char encoded portion is valid for both algorithms per the
				// loose digest grammar in this codebase. We reuse the sha256
				// encoded portion to make the on-disk file accessible by hex.
				sha512Dgst := domain.Digest("sha512:" + encoded(sha256Dgst))

				mf := manifestWith(sha256Dgst, nil, "")

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2",
					encoded(sha256Dgst), blobBody)

				// The file exists at <tag>/<hex>, but the manifest references
				// sha256:<hex>, not sha512:<hex>. Full-digest comparison must
				// reject this request.
				_, err := puller.PullBlob(context.Background(), image, sha512Dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("client requests sha256:<hex> but manifest only references sha512:<same-hex>", func() {
			It("returns ErrBlobNotFound (no cross-algorithm match)", func() {
				sha256Dgst := sha256Digest(blobBody)
				sha512Dgst := domain.Digest("sha512:" + encoded(sha256Dgst))

				mf := manifestWith(sha512Dgst, nil, "")

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), mf)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2",
					encoded(sha256Dgst), blobBody)

				_, err := puller.PullBlob(context.Background(), image, sha256Dgst)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})
	})

	Context("Miss paths", func() {
		When("the walker yields no entries", func() {
			It("returns ErrBlobNotFound with BLOB_UNKNOWN", func() {
				digest := sha256Digest(blobBody)

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("the walker yields an entry but no blob file exists at that tag", func() {
			It("returns ErrBlobNotFound (silent stat-miss)", func() {
				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				// note: no writeBlobFile

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("the blob file exists but ReadManifestBytes errors", func() {
			It("skips the entry and returns ErrBlobNotFound", func() {
				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)
				mockWalker.SetReadError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("the blob file exists but the manifest is invalid JSON", func() {
			It("skips the entry and returns ErrBlobNotFound", func() {
				digest := sha256Digest(blobBody)

				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), []byte("not json"))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("the blob file exists but the manifest fails domain validation", func() {
			It("skips the entry and returns ErrBlobNotFound", func() {
				digest := sha256Digest(blobBody)
				// schemaVersion 1 fails Manifest.Validate
				badBytes := []byte(`{"schemaVersion":1}`)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), badBytes)
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})
	})

	Context("Terminal errors", func() {
		When("the walker yields an error", func() {
			It("propagates the wrapped error", func() {
				mockWalker.SetWalkError(errors.Wrap(domain.ErrRegistryInternal))

				_, err := puller.PullBlob(context.Background(), image, sha256Digest(blobBody))
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrRegistryInternal))
			})
		})
	})

	Context("Non-ENOENT filesystem errors", func() {
		When("the tag directory is unreadable", func() {
			It("skips the entry and returns ErrBlobNotFound", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				tagDir := filepath.Join(suite.FsRoot, "sol-a", "v1.0.0", string(image), "3.22.2")
				Expect(os.Chmod(tagDir, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(tagDir, utils.PermissionOK) }()

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})

		When("the blob file is unreadable", func() {
			It("skips the entry and returns ErrBlobNotFound", func() {
				if os.Geteuid() == 0 {
					Skip("permission-based test skipped when running as root")
				}

				digest := sha256Digest(blobBody)
				mockWalker.AddEntry(entry("sol-a", "v1.0.0", "3.22.2"), manifestWith(digest, nil, ""))
				writeBlobFile(suite.FsRoot, "sol-a", "v1.0.0", image, "3.22.2", encoded(digest), blobBody)

				blobFile := filepath.Join(
					suite.FsRoot, "sol-a", "v1.0.0", string(image), "3.22.2", encoded(digest),
				)
				Expect(os.Chmod(blobFile, utils.PermissionNone)).To(Succeed())

				defer func() { _ = os.Chmod(blobFile, utils.PermissionOK) }()

				_, err := puller.PullBlob(context.Background(), image, digest)
				Expect(err).To(HaveOccurred())
				utils.ValidateError(err)
				Expect(err).To(MatchError(domain.ErrBlobNotFound))
			})
		})
	})
})
