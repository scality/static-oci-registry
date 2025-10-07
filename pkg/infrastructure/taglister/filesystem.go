//nolint:cyclop // this is infrastructure code and is inherently complex
package taglister

import (
	"os"
	"slices"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/infrastructure/imagefinder"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type FileSystem struct {
	logger      *zerolog.Logger
	imageFinder *imagefinder.FileSystem
	fsRoot      string
}

func NewFileSystem(
	l *zerolog.Logger,
	i *imagefinder.FileSystem,
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

	return &FileSystem{
		logger:      l,
		imageFinder: i,
		fsRoot:      r,
	}, nil
}

func (fs *FileSystem) ListTags(imageName domain.ImageName) (*domain.ListTagsOutput, error) {
	l := fs.logger.With().Str("image_name", string(imageName)).Logger()
	l.Info().Msg("Finding tags in filesystem registry")
	// the registry is stored in the filesystem at fsRoot
	candidates, err := fs.imageFinder.FindImage(imageName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find image in filesystem registry")
	}

	allTags := make([]domain.Tag, 0, len(candidates))

	for _, dir := range candidates {
		tagEntries, err := os.ReadDir(dir)
		if err != nil {
			return nil, errors.Wrap(domain.ErrRegistryInternal,
				"failed to read an image dir in filesystem registry")
		}

		for _, entry := range tagEntries {
			if !entry.IsDir() {
				continue
			}

			// make sure this directory contains a manifest.json file
			manifestPath := dir + "/" + entry.Name() + "/manifest.json"
			if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
				// we don't pollute the logs in this case
				// since it could be an intermediary directory
				continue
			}

			tag := domain.Tag(entry.Name())
			tl := l.With().Str("tag", string(tag)).Logger()

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
