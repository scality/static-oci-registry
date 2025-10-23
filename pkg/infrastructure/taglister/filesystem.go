// nolint:cyclop // this is complex infrastructure logic by nature
package taglister

import (
	"os"
	"slices"

	"github.com/scality/static-oci-registry/pkg/domain"
	apperrors "github.com/scality/static-oci-registry/pkg/errors"
	"github.com/scality/static-oci-registry/pkg/service"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type FileSystem struct {
	logger      *zerolog.Logger
	imageFinder service.ImageFinder
	fsRoot      string
}

func NewFileSystem(
	l *zerolog.Logger,
	i service.ImageFinder,
	r string,
) (*FileSystem, error) {
	// make sure r exists
	info, err := os.Stat(r)
	if err != nil {
		return nil, errors.Wrap(err, "unable to access FS_ROOT")
	}

	// make sure r is a directory
	if !info.IsDir() {
		return nil, errors.New("passed FS_ROOT is not a directory")
	}

	logger := l.With().Str("taglister", "filesystem").Logger()

	return &FileSystem{
		logger:      &logger,
		imageFinder: i,
		fsRoot:      r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) ListTags(imageName domain.ImageName) (*domain.ListTagsOutput,
	*apperrors.Error,
) {
	l := fs.logger.With().Str("image_name", string(imageName)).Logger()
	l.Info().Msg("Finding tags in filesystem registry")
	// the registry is stored in the filesystem at fsRoot
	candidates, err := fs.imageFinder.FindImage(imageName)
	if err != nil {
		return nil, err.Wrap("failed to find image in filesystem registry")
	}

	allTags := make([]domain.Tag, 0, len(candidates))

	for _, sv := range candidates {
		dir := fs.fsRoot +
			"/" + sv.Solution +
			"/" + sv.Version +
			"/" + string(imageName)

		tagEntries, err := os.ReadDir(dir)
		if err != nil {
			return nil, apperrors.New(domain.ErrRegistryInternal, nil).
				WrapErr(err).
				Wrap("failed to read an image dir in filesystem registry")
		}

		for _, entry := range tagEntries {
			if !entry.IsDir() {
				continue
			}

			// make sure this directory contains a manifest.json file
			manifestPath := dir + "/" + entry.Name() + "/manifest.json"
			if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
				l.Warn().Str("location", dir).Str("tag", entry.Name()).
					Msg("location/tag directory does not contain manifest.json file")

				continue
			}

			tag := domain.Tag(entry.Name())
			tl := l.With().Str("location", dir).Str("tag", string(tag)).Logger()

			err := tag.Validate()
			if err != nil {
				tl.Warn().Err(err).Msg("invalid tag found, skipping")

				continue
			}

			if slices.Contains(allTags, tag) {
				tl.Warn().Msg("duplicate tag found, skipping")

				continue
			}

			allTags = append(allTags, tag)
		}
	}

	// sort Tags
	slices.SortFunc(allTags, domain.CompareTags)

	return &domain.ListTagsOutput{Name: imageName, Tags: allTags}, nil
}
