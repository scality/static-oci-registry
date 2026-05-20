// nolint:cyclop // this is complex infrastructure logic by nature
package imagefinder

import (
	"os"
	"sort"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"

	"github.com/hashicorp/go-version"
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
		return nil, errors.Wrap(err, errors.WithDetail("unable to access FS_ROOT"))
	}

	// make sure r is a directory
	if !info.IsDir() {
		return nil, errors.New("passed FS_ROOT is not a directory")
	}

	logger := l.With().Str("imagefinder", "filesystem").Logger()

	return &FileSystem{
		logger: &logger,
		fsRoot: r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FindImage(imageName domain.ImageName) ([]domain.SolutionVersion,
	error,
) {
	l := fs.logger.With().Str("image_name", string(imageName)).Logger()
	l.Info().Msg("Finding image in filesystem registry")
	// walks the fsroot by solution and version and gathers a list of `solution/version` dirs
	// such that `solution/version/imageName` exists, is a dir, and is not empty
	// return an error if no such directory exists
	solutions, err := os.ReadDir(fs.fsRoot)
	if err != nil {
		return nil, errors.Wrap(
			domain.ErrRegistryInternal,
			errors.CausedBy(err),
			errors.WithDetail("failed to read fs root in filesystem registry"),
		)
	}

	found := make([]domain.SolutionVersion, 0)

	for _, solution := range solutions {
		if !solution.IsDir() {
			l.Warn().Str("solution", solution.Name()).
				Msg("solutions in the root of the filesystem should all be directories")

			continue
		}

		versionCandidates, err := os.ReadDir(fs.fsRoot + "/" + solution.Name())
		if err != nil {
			return nil, errors.Wrap(
				domain.ErrRegistryInternal,
				errors.CausedBy(err),
				errors.WithDetail("failed to read solution dir in filesystem registry"),
			)
		}

		versions := make([]os.DirEntry, 0)
		unsortedVersions := make([]os.DirEntry, 0)

		for _, candidate := range versionCandidates {
			if !candidate.IsDir() {
				l.Warn().Str("solution", solution.Name()).Str("version", candidate.Name()).
					Msg("solution/version should be a directory")

				continue
			}

			// validate that candidate.Name() is a valid semver
			_, err := version.NewVersion(candidate.Name())
			if err != nil {
				l.Warn().Err(err).Str("solution", solution.Name()).
					Str("version", candidate.Name()).
					Msg("invalid version directory name found in solution")

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
					return nil, errors.Wrap(
						domain.ErrRegistryInternal,
						errors.CausedBy(err),
						errors.WithDetail("failed to stat image dir in filesystem registry"),
					)
				}

				continue
			}

			if !info.IsDir() {
				l.Warn().Str("solution", solution.Name()).Str("version", candidate.Name()).
					Msg("solution/version/image_name should be a directory")

				continue
			}

			found = append(found, domain.SolutionVersion{
				Solution: solution.Name(),
				Version:  candidate.Name(),
			})
		}
	}

	if len(found) == 0 {
		return nil, errors.Wrap(
			domain.ErrImageNotFound,
			errors.WithDetail("image not found in filesystem registry"),
			errors.WithProperty(ocierrors.OCICode, ocierrors.NameUnknown),
			errors.WithProperty(ocierrors.OCIMessage, domain.ErrImageNotFound.Error()),
			errors.WithProperty(ocierrors.OCIPrefix+"image_name", string(imageName)),
		)
	}

	return found, nil
}
