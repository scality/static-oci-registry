package digestmanifestfetcher

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"hash"
	"log/slog"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

// supportedAlgorithms maps each supported digest algorithm to a constructor
// for its hash.Hash. Real registries (Quay, MCR) return MANIFEST_UNKNOWN for
// digests using algorithms they don't support; we mirror that behavior but
// short-circuit the tag walk to avoid pointless I/O.
var supportedAlgorithms = map[string]func() hash.Hash{
	"sha256": sha256.New,
	"sha512": sha512.New,
}

type FileSystem struct {
	logger    *slog.Logger
	tagWalker service.TagWalker
}

func NewFileSystem(
	l *slog.Logger,
	t service.TagWalker,
) (*FileSystem, error) {
	return &FileSystem{
		logger:    l.With(slog.String("digest_manifest_fetcher", "filesystem")),
		tagWalker: t,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FetchManifest(
	ctx context.Context,
	imageName domain.ImageName,
	digest domain.Digest,
) (*domain.FetchManifestOutput, error) {
	l := fs.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Finding manifest in filesystem registry")

	// Check algorithm support up front: an unsupported algorithm can never
	// match any of our manifests, so there is no point walking the tags.
	// Real registries (Quay, MCR) return MANIFEST_UNKNOWN in this case,
	// so we do the same.
	algorithm, err := digest.Algorithm()
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to get digest algorithm"))
	}

	newHash, supported := supportedAlgorithms[algorithm]
	if !supported {
		return nil, errors.Wrap(
			domain.ErrManifestNotFound,
			errors.WithDetail("unsupported digest algorithm"),
			ocierrors.BuildOCIProperties(
				ocierrors.ManifestUnknown,
				domain.ErrManifestNotFound.Error(),
				map[string]string{
					"digest":    digest.String(),
					"algorithm": algorithm,
				},
			),
		)
	}

	encoded, err := digest.Encoded()
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to get digest encoded value"))
	}

	for entry, err := range fs.tagWalker.WalkTags(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("failure while walking tags in filesystem registry"),
			)
		}

		manifestBytes, err := fs.tagWalker.ReadManifestBytes(entry)
		if err != nil {
			// Soft-fail: skip unreadable manifests; if none match the caller
			// will return MANIFEST_UNKNOWN.
			l.WarnContext(ctx, "failed to read manifest bytes, skipping tag entry",
				slog.String("solution", entry.Solution),
				slog.String("version", entry.Version),
				slog.String("tag", entry.Tag.String()),
				slog.Any("error", err),
			)

			continue
		}

		var manifest domain.Manifest

		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			l.WarnContext(ctx, "failed to unmarshal manifest, skipping tag entry",
				slog.String("solution", entry.Solution),
				slog.String("version", entry.Version),
				slog.String("tag", entry.Tag.String()),
				slog.Any("error", err),
			)

			continue
		}

		if err := manifest.Validate(); err != nil {
			l.WarnContext(ctx, "invalid manifest contents, skipping tag entry",
				slog.String("solution", entry.Solution),
				slog.String("version", entry.Version),
				slog.String("tag", entry.Tag.String()),
				slog.Any("error", err),
			)

			continue
		}

		match := digestMatchesManifest(newHash, encoded, manifestBytes)
		if !match {
			continue
		}

		return &domain.FetchManifestOutput{
			MediaType:     manifest.MediaType,
			ContentDigest: digest,
			ManifestBytes: manifestBytes,
		}, nil
	}

	return nil, errors.Wrap(
		domain.ErrManifestNotFound,
		errors.WithDetail("manifest not found in filesystem registry"),
		ocierrors.BuildOCIProperties(
			ocierrors.ManifestUnknown,
			domain.ErrManifestNotFound.Error(),
			map[string]string{
				"digest": digest.String(),
			},
		),
	)
}

// digestMatchesManifest reports whether `manifestBytes` hashes to `encoded`
// under the algorithm whose hash.Hash is produced by `newHash`. hash.Hash.Write
// never returns an error (per its contract), so this function cannot fail.
func digestMatchesManifest(newHash func() hash.Hash, encoded string, manifestBytes []byte) bool {
	h := newHash()
	h.Write(manifestBytes)

	return hex.EncodeToString(h.Sum(nil)) == encoded
}
