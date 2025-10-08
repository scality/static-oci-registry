// nolint:cyclop // this is complex infrastructure logic by nature
package imagefinder

import (
	"os"
	"sort"

	"github.com/scality/static-oci-registry/pkg/domain"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type FileSystem struct {
	logger *zerolog.Logger
	fsRoot string
}

func NewFileSystem(
	l *zerolog.Logger,
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
		logger: l,
		fsRoot: r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FindImage(imageName domain.ImageName) ([]string, error) {
	l := fs.logger.With().Str("image_name", string(imageName)).Logger()
	l.Info().Msg("Finding image in filesystem registry")
	// walks the fsroot by solution and version and gathers a list of `solution/version` dirs
	// such that `solution/version/imageName` exists, is a dir, and is not empty
	// return an error if no such directory exists
	solutions, err := os.ReadDir(fs.fsRoot)
	if err != nil {
		// maybe use wrapf
		return nil, errors.Wrap(domain.ErrRegistryInternal,
			"failed to read fs root in filesystem registry")
	}

	found := make([]string, 0)

	for _, solution := range solutions {
		if !solution.IsDir() {
			continue
		}

		versionCandidates, err := os.ReadDir(fs.fsRoot + "/" + solution.Name())
		if err != nil {
			return nil, errors.Wrap(domain.ErrRegistryInternal,
				"failed to read solution dir in filesystem registry")
		}

		versions := make([]os.DirEntry, 0)
		unsortedVersions := make([]os.DirEntry, 0)

		for _, candidate := range versionCandidates {
			if !candidate.IsDir() {
				continue
			}

			// validate that candidate.Name() is a valid semver
			_, err := version.NewVersion(candidate.Name())
			if err != nil {
				l.Warn().Err(err).Str("version", candidate.Name()).
					Msg("invalid version directory, skipping")

				unsortedVersions = append(unsortedVersions, candidate)

				continue
			}

			versions = append(versions, candidate)
		}

		sort.Slice(versions, func(i, j int) bool {
			// name is already validated, panic if we can't instantiate a version
			vi := version.Must(version.NewVersion(versions[i].Name()))
			vj := version.Must(version.NewVersion(versions[j].Name()))

			return vi.LessThan(vj)
		})

		// we include invalid semvers either way as a fallback
		versions = append(versions, unsortedVersions...)

		for _, candidate := range versions {
			info, err := os.Stat(
				fs.fsRoot +
					"/" + solution.Name() +
					"/" + candidate.Name() +
					"/" + string(imageName),
			)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, errors.Wrap(domain.ErrRegistryInternal,
						"failed to stat image dir in filesystem registry")
				}

				continue
			}

			if info.IsDir() {
				found = append(found,
					fs.fsRoot+
						"/"+solution.Name()+
						"/"+candidate.Name()+
						"/"+string(imageName),
				)
			}
		}
	}

	if len(found) == 0 {
		return nil, errors.Wrap(domain.ErrImageNotFound,
			"no tags found for image in filesystem registry")
	}

	return found, nil
}
