package tagwalker

import (
	"context"
	iofs "io/fs"
	"iter"
	"log/slog"
	"os"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/service"
)

const manifestFileName = "manifest.json"

type FileSystem struct {
	logger      *slog.Logger
	imageFinder service.ImageFinder
	root      *os.Root
}

func NewFileSystem(
	logger *slog.Logger,
	imageFinder service.ImageFinder,
	root *os.Root,
) (*FileSystem, error) {
	return &FileSystem{
		logger:      logger.With(slog.String("tag_walker", "filesystem")),
		imageFinder: imageFinder,
		root:      root,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) WalkTags(
	ctx context.Context,
	imageName domain.ImageName,
) iter.Seq2[domain.TagEntry, error] {
	return func(yield func(domain.TagEntry, error) bool) {
		l := fs.logger.With(slog.String("image_name", string(imageName)))
		l.InfoContext(ctx, "Walking tags in filesystem registry")

		candidates, err := fs.imageFinder.FindImage(ctx, imageName)
		if err != nil {
			yield(domain.TagEntry{},
				errors.Wrap(err, errors.WithDetail("failed to find image in filesystem registry")))

			return
		}

		for _, sv := range candidates {
			dir := strings.Join([]string{sv.Solution, sv.Version, imageName.String()}, "/")

			tagEntries, err := iofs.ReadDir(fs.root.FS(), dir)
			if err != nil {
				// Soft-fail: a single unreadable image dir shouldn't abort the
				// whole walk. Log and skip; if no candidate yields a match the
				// caller will return a normal not-found.
				l.WarnContext(ctx, "failed to read an image dir, skipping",
					slog.String("location", dir),
					slog.Any("error", err),
				)

				continue
			}

			for _, entry := range tagEntries {
				if !entry.IsDir() {
					continue
				}

				tag := domain.Tag(entry.Name())
				tl := l.With(
					slog.String("location", dir),
					slog.String("tag", string(tag)),
				)

				err := tag.Validate()
				if err != nil {
					tl.WarnContext(ctx, "invalid tag found, skipping",
						slog.Any("error", err),
					)

					continue
				}

				// make sure this directory contains a manifest.json file
				tagEntry := domain.TagEntry{SolutionVersion: sv, Name: imageName, Tag: tag}
				if info, err := fs.root.Stat(manifestPath(tagEntry)); err != nil || info.IsDir() {
					tl.WarnContext(ctx, "location/tag directory does not contain manifest file")

					continue
				}

				if !yield(tagEntry, nil) {
					return
				}
			}
		}
	}
}

func (fs *FileSystem) ReadManifestBytes(entry domain.TagEntry) ([]byte, error) {
	bytes, err := fs.root.ReadFile(manifestPath(entry))
	if err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to read manifest file in filesystem registry"))
	}

	return bytes, nil
}

func manifestPath(entry domain.TagEntry) string {
	return strings.Join([]string{
		entry.Solution, entry.Version, entry.Name.String(), entry.Tag.String(), manifestFileName,
	}, "/")
}
