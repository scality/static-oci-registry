package tagwalker

import (
	"context"
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
		logger:      l.With(slog.String("tag_walker", "filesystem")),
		imageFinder: i,
		fsRoot:      r,
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
			dir := fs.fsRoot +
				"/" + sv.Solution +
				"/" + sv.Version +
				"/" + string(imageName)

			tagEntries, err := os.ReadDir(dir)
			if err != nil {
				yield(domain.TagEntry{}, errors.Wrap(
					domain.ErrRegistryInternal,
					errors.CausedBy(err),
					errors.WithDetail("failed to read an image dir in filesystem registry"),
				))

				return
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
				if info, err := os.Stat(fs.manifestPath(tagEntry)); err != nil || info.IsDir() {
					l.WarnContext(ctx, "location/tag directory does not contain manifest file",
						slog.String("location", dir),
						slog.String("tag", entry.Name()),
					)

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
	bytes, err := os.ReadFile(fs.manifestPath(entry))
	if err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to read manifest file in filesystem registry"))
	}

	return bytes, nil
}

func (fs *FileSystem) manifestPath(entry domain.TagEntry) string {
	return strings.Join([]string{
		fs.fsRoot, entry.Solution, entry.Version, entry.Name.String(),
		entry.Tag.String(), manifestFileName,
	}, "/")
}
