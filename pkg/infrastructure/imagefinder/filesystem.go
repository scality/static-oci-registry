// nolint:cyclop // this is complex infrastructure logic by nature
package imagefinder

import (
	"context"
	"log/slog"
	"os"
	"sort"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	apperrors "github.com/scality/static-oci-registry/pkg/errors"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
)

type FileSystem struct {
	logger *slog.Logger
	fsRoot string
}

func NewFileSystem(
	l *slog.Logger,
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
		logger: l.With(slog.String("image_finder", "filesystem")),
		fsRoot: r,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FindImage(ctx context.Context, imageName domain.ImageName) (
	[]domain.SolutionVersion, *apperrors.Error,
) {
	l := fs.logger.With(slog.String("image_name", string(imageName)))
	l.InfoContext(ctx, "Finding image in filesystem registry")
	// walks the fsroot by solution and version and gathers a list of `solution/version` dirs
	// such that `solution/version/imageName` exists, is a dir, and is not empty
	// return an error if no such directory exists
	solutions, err := os.ReadDir(fs.fsRoot)
	if err != nil {
		return nil, apperrors.New(domain.ErrRegistryInternal, nil).
			WrapErr(err).
			Wrap("failed to read fs root in filesystem registry")
	}

	found := make([]domain.SolutionVersion, 0)

	for _, solution := range solutions {
		if !solution.IsDir() {
			l.WarnContext(ctx, "solutions in the root of the filesystem should all be directories",
				slog.String("solution", solution.Name()),
			)

			continue
		}

		versionCandidates, err := os.ReadDir(fs.fsRoot + "/" + solution.Name())
		if err != nil {
			return nil, apperrors.New(domain.ErrRegistryInternal, nil).
				WrapErr(err).
				Wrap("failed to read solution dir in filesystem registry")
		}

		versions := make([]os.DirEntry, 0)
		unsortedVersions := make([]os.DirEntry, 0)

		for _, candidate := range versionCandidates {
			if !candidate.IsDir() {
				l.WarnContext(ctx, "solution/version should be a directory",
					slog.String("solution", solution.Name()),
					slog.String("version", candidate.Name()),
				)

				continue
			}

			// validate that candidate.Name() is a valid semver
			_, err := version.NewVersion(candidate.Name())
			if err != nil {
				l.WarnContext(ctx, "invalid version directory name found in solution",
					slog.String("solution", solution.Name()),
					slog.String("version", candidate.Name()),
					slog.Any("error", err),
				)

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
					return nil, apperrors.New(domain.ErrRegistryInternal, nil).
						WrapErr(err).
						Wrap("failed to stat image dir in filesystem registry")
				}

				continue
			}

			if !info.IsDir() {
				l.WarnContext(ctx, "solution/version/image_name should be a directory",
					slog.String("solution", solution.Name()),
					slog.String("version", candidate.Name()),
				)

				continue
			}

			found = append(found, domain.SolutionVersion{
				Solution: solution.Name(),
				Version:  candidate.Name(),
			})
		}
	}

	if len(found) == 0 {
		return nil, apperrors.FromCode(domain.ErrImageNotFound, ocierrors.NameUnknown).
			Wrap("image not found in filesystem registry").
			WithOCIMessage(domain.ErrImageNotFound.Error()).
			WithOCIDetail("image_name", string(imageName))
	}

	return found, nil
}
