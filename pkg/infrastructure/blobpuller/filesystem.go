// nolint:cyclop // this is complex infrastructure logic by nature
package blobpuller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"
)

type FileSystem struct {
	logger    *slog.Logger
	tagWalker service.TagWalker
	fsRoot    string
}

func NewFileSystem(
	l *slog.Logger,
	t service.TagWalker,
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
		logger:    l.With(slog.String("blob_puller", "filesystem")),
		tagWalker: t,
		fsRoot:    r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) PullBlob(
	ctx context.Context,
	imageName domain.ImageName,
	digest domain.Digest,
) (io.ReadSeekCloser, error) {
	l := fs.logger.With(
		slog.String("image_name", string(imageName)),
		slog.String("digest", string(digest)),
	)
	l.InfoContext(ctx, "Pulling blob from filesystem registry")

	for entry, err := range fs.tagWalker.WalkTags(ctx, imageName) {
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("failure while walking tags in filesystem registry"),
			)
		}

		path := fs.blobPath(entry, digest)

		// Cheap existence check: if the file isn't here, skip the manifest
		// parse entirely and move on to the next candidate tag.
		if _, err := os.Stat(path); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				l.WarnContext(ctx, "failed to stat blob file, skipping tag",
					slog.String("solution", entry.Solution),
					slog.String("version", entry.Version),
					slog.String("tag", entry.Tag.String()),
					slog.Any("error", err),
				)
			}

			continue
		}

		// The file exists. Confirm the manifest at this tag authorizes
		// serving it — this is what enforces both algorithm correctness
		// (full digest comparison) and prevents leaking stray files that
		// happen to live in the tag directory but aren't part of the image.
		ok, err := fs.manifestReferences(entry, digest)
		if err != nil {
			l.WarnContext(ctx, "failed to verify blob is referenced by manifest, skipping tag",
				slog.String("solution", entry.Solution),
				slog.String("version", entry.Version),
				slog.String("tag", entry.Tag.String()),
				slog.Any("error", err),
			)

			continue
		}

		if !ok {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			l.WarnContext(ctx, "failed to open blob file after stat, skipping tag",
				slog.String("solution", entry.Solution),
				slog.String("version", entry.Version),
				slog.String("tag", entry.Tag.String()),
				slog.Any("error", err),
			)

			continue
		}

		return file, nil
	}

	return nil, errors.Wrap(
		domain.ErrBlobNotFound,
		errors.WithDetail("blob not found in filesystem registry"),
		ocierrors.BuildOCIProperties(
			ocierrors.BlobUnknown,
			domain.ErrBlobNotFound.Error(),
			map[string]string{
				"digest": string(digest),
			},
		),
	)
}

// blobPath returns the on-disk location of the blob with the given encoded
// digest for the given tag entry. The layout mirrors tagwalker.manifestPath
// so both helpers stay in sync.
func (fs *FileSystem) blobPath(entry domain.TagEntry, digest domain.Digest) string {
	encoded, err := digest.Encoded()
	if err != nil {
		// Unreachable: Encoded is a regex-capture wrapper that uses the same
		// pattern as Digest.Validate, and the digest is validated in the HTTP
		// handler before reaching this layer. Failure here means an upstream
		// caller bypassed validation — a programming error, not a client one.
		panic(fmt.Sprintf("blob_puller: unvalidated digest %q reached infrastructure layer", digest))
	}

	return strings.Join([]string{
		fs.fsRoot, entry.Solution, entry.Version, entry.Name.String(),
		entry.Tag.String(), encoded,
	}, "/")
}

// manifestReferences reports whether the manifest at the given tag entry
// references the given digest as its config, one of its layers, or its
// subject. Comparison is on the full algo:encoded form, which is what
// enforces algorithm matching (a sha256 manifest entry will never satisfy
// a sha512 request).
func (fs *FileSystem) manifestReferences(
	entry domain.TagEntry,
	digest domain.Digest,
) (bool, error) {
	manifestBytes, err := fs.tagWalker.ReadManifestBytes(entry)
	if err != nil {
		return false, errors.Wrap(err,
			errors.WithDetail("failed to read manifest bytes in filesystem registry"))
	}

	var manifest domain.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return false, errors.Wrap(err,
			errors.WithDetail("failed to unmarshal manifest in filesystem registry"))
	}

	if err := manifest.Validate(); err != nil {
		return false, errors.Wrap(err,
			errors.WithDetail("invalid manifest contents in filesystem registry"))
	}

	if manifest.Config != nil && digest == manifest.Config.Digest {
		return true, nil
	}

	for _, layer := range manifest.Layers {
		if digest == layer.Digest {
			return true, nil
		}
	}

	if manifest.Subject != nil && digest == manifest.Subject.Digest {
		return true, nil
	}

	return false, nil
}
