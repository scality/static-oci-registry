package digestmanifestfetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger      *slog.Logger
	imageFinder service.ImageFinder
	fsRoot      string
}

func NewFileSystem(
	l *slog.Logger,
	i service.ImageFinder,
	r string,
) (*FileSystem, error) {
	info, err := os.Stat(r)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("unable to access FS_ROOT"))
	}

	if !info.IsDir() {
		return nil, errors.New("passed FS_ROOT is not a directory")
	}

	return &FileSystem{
		logger:      l.With(slog.String("digest_manifest_fetcher", "filesystem")),
		imageFinder: i,
		fsRoot:      r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FetchManifest(ctx context.Context, imageName domain.ImageName, digest domain.Digest) (
	*domain.FetchManifestOutput, error,
) {
	l := fs.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Finding manifest in filesystem registry")

	candidates, err := fs.imageFinder.FindImage(ctx, imageName)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed to find image in filesystem registry"))
	}

	for _, sv := range candidates {
		dir := fs.fsRoot +
			"/" + sv.Solution +
			"/" + sv.Version +
			"/" + string(imageName)

		tagEntries, err := os.ReadDir(dir)
		if err != nil {
			return nil, errors.Wrap(
				domain.ErrRegistryInternal,
				errors.CausedBy(err),
				errors.WithDetail("failed to read an image dir in filesystem registry"),
			)
		}

		for _, tagEntry := range tagEntries {
			if !tagEntry.IsDir() {
				continue
			}

			manifestPath := dir + "/" + tagEntry.Name() + "/manifest.json"
			if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
				l.WarnContext(ctx, "skipping tag entry as it does not contain a manifest.json file",
					slog.String("location", dir),
					slog.String("tag", tagEntry.Name()),
				)

				continue
			}

			// load manifest content
			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				l.WarnContext(ctx, "skipping tag entry as manifest.json file could not be read",
					slog.String("location", manifestPath),
				)

				continue
			}

			var manifest domain.Manifest

			err = json.Unmarshal(manifestBytes, &manifest)
			if err != nil {
				l.WarnContext(ctx, "skipping tag entry as manifest.json file could not be unmarshalled",
					slog.String("location", manifestPath),
				)

				continue
			}

			if err := manifest.Validate(); err != nil {
				l.WarnContext(ctx, "skipping tag entry as manifest.json file is not a valid manifest",
					slog.String("location", manifestPath),
				)

				continue
			}

			match, err := digestMatchesManifest(digest, manifestBytes)
			if err != nil {
				l.WarnContext(ctx, "skipping tag entry as manifest digest could not be compared",
					slog.Any("error", err),
					slog.String("location", manifestPath),
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
