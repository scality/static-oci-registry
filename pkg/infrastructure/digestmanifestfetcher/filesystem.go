package digestmanifestfetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

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

	for entry, err := range fs.tagWalker.WalkTags(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("failure while walking tags in filesystem registry"),
			)
		}

		manifestBytes, err := fs.tagWalker.ReadManifestBytes(entry)
		if err != nil {
			return nil, errors.Wrap(
				domain.ErrRegistryInternal,
				errors.CausedBy(err),
				errors.WithDetail("failed to read manifest bytes while walking tags in filesystem registry"),
			)
		}

		var manifest domain.Manifest

		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return nil, errors.Wrap(
				domain.ErrRegistryInternal,
				errors.CausedBy(err),
				errors.WithDetail("failed to unmarshal manifest while walking tags in filesystem registry"),
			)
		}

		if err := manifest.Validate(); err != nil {
			return nil, errors.Wrap(
				domain.ErrRegistryInternal,
				errors.CausedBy(err),
				errors.WithDetail("invalid manifest contents while walking tags in filesystem registry"),
			)
		}

		match, err := digestMatchesManifest(digest, manifestBytes)
		if err != nil {
			l.WarnContext(ctx, "skipping tag entry as manifest digest could not be compared",
				slog.Any("error", err),
				slog.String("digest", string(digest)),
			)

			continue
		}

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

func digestMatchesManifest(digest domain.Digest, manifestBytes []byte) (bool, error) {
	var sum []byte

	algorithm, err := digest.Algorithm()
	if err != nil {
		return false, errors.Wrap(err, errors.WithDetail("failed to get digest algorithm"))
	}

	encoded, err := digest.Encoded()
	if err != nil {
		return false, errors.Wrap(err, errors.WithDetail("failed to get digest encoded value"))
	}

	switch algorithm {
	case "sha256":
		h := sha256.Sum256(manifestBytes)
		sum = h[:]
	default:
		return false, errors.New("unsupported hashing algorithm")
	}

	return hex.EncodeToString(sum) == encoded, nil
}
