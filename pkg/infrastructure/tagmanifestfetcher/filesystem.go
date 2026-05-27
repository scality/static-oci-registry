package tagmanifestfetcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
		logger:      l.With(slog.String("tag_manifest_fetcher", "filesystem")),
		imageFinder: i,
		fsRoot:      r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FetchManifest(ctx context.Context, imageName domain.ImageName, tag domain.Tag) (
	*domain.FetchManifestOutput, error,
) {
	l := fs.logger.With(slog.String("image_name", string(imageName)), slog.String("tag", string(tag)))
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
			if tagEntry.Name() != string(tag) {
				continue
			}

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

			hash := sha256.Sum256(manifestBytes)

			return &domain.FetchManifestOutput{
				MediaType:     manifest.MediaType,
				ContentDigest: domain.Digest(fmt.Sprintf("sha256:%x", string(hash[:]))),
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
				"tag": tag.String(),
			},
		),
	)
}
