package tagmanifestfetcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
		logger:    l.With(slog.String("tag_manifest_fetcher", "filesystem")),
		tagWalker: t,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FetchManifest(
	ctx context.Context,
	imageName domain.ImageName,
	tag domain.Tag,
) (*domain.FetchManifestOutput, error) {
	l := fs.logger.With(slog.String("image_name", string(imageName)), slog.String("tag", string(tag)))
	l.InfoContext(ctx, "Finding manifest in filesystem registry")

	for entry, err := range fs.tagWalker.WalkTags(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("failure while walking tags in filesystem registry"),
			)
		}

		if entry.Tag != tag {
			continue
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

		hash := sha256.Sum256(manifestBytes)

		return &domain.FetchManifestOutput{
			MediaType:     manifest.MediaType,
			ContentDigest: domain.Digest(fmt.Sprintf("sha256:%x", string(hash[:]))),
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
				"tag": tag.String(),
			},
		),
	)
}
