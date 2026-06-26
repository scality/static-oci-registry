// nolint:cyclop // this is complex infrastructure logic by nature
package imagefinder

import (
	"context"
	iofs "io/fs"
	"log/slog"
	"os"
	"sort"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/service"

	"github.com/hashicorp/go-version"
)

type FileSystem struct {
	logger *slog.Logger
	root   *os.Root
}

var _ service.ImageFinder = (*FileSystem)(nil)

func NewFileSystem(
	logger *slog.Logger,
	root *os.Root,
) (*FileSystem, error) {
	// make sure r exists
	return &FileSystem{
		logger: logger.With(slog.String("image_finder", "filesystem")),
		root:   root,
	}, nil
}

// nolint:gocognit,funlen // this is the core function of this service
// and can not be split meaningfully.
func (fs *FileSystem) FindImage(ctx context.Context, imageName domain.ImageName) (
	[]domain.SolutionVersion, error,
) {
	l := fs.logger.With(slog.String("image_name", string(imageName)))
	l.InfoContext(ctx, "Finding image in filesystem registry")
	// walks the fsroot by solution and version and gathers a list of `solution/version` dirs
	// such that `solution/version/imageName` exists, is a dir, and is not empty
	// return an error if no such directory exists
	solutions, err := iofs.ReadDir(fs.root.FS(), ".")
	if err != nil {
		return nil, errors.Wrap(
			domain.ErrRegistryInternal,
			errors.CausedBy(err),
			errors.WithDetail("failed to read fs root in filesystem registry"),
		)
	}

	found := make(map[string][]domain.SolutionVersion, 0)

	for _, solution := range solutions {
		if !solution.IsDir() {
			l.WarnContext(ctx, "solutions in the root of the filesystem should all be directories",
				slog.String("solution", solution.Name()),
			)

			continue
		}

		found[solution.Name()] = make([]domain.SolutionVersion, 0)

		versionCandidates, err := iofs.ReadDir(fs.root.FS(), solution.Name())
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

			return vj.LessThan(vi)
		})

		// we include invalid semvers either way as a fallback
		versions = append(versions, unsortedVersions...)

		for _, candidate := range versions {
			info, err := fs.root.Stat(
				solution.Name() +
					"/" + candidate.Name() +
					"/" + string(imageName),
			)
			if err != nil {
				if !os.IsNotExist(err) {
					// Soft-fail: a single unstattable image dir shouldn't
					// abort the whole search. Log and skip; if no candidate
					// exists at all the caller returns NAME_UNKNOWN.
					l.WarnContext(ctx, "failed to stat image dir, skipping candidate",
						slog.String("solution", solution.Name()),
						slog.String("version", candidate.Name()),
						slog.Any("error", err),
					)
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

			base := solution.Name() + "/" + candidate.Name() + "/" + string(imageName)
			if !fs.isLayout(ctx, l, base) {
				continue
			}

			found[solution.Name()] = append(found[solution.Name()], domain.SolutionVersion{
				Solution: solution.Name(),
				Version:  candidate.Name(),
			})
		}
	}

	sorted := make([]domain.SolutionVersion, 0)

	finished := false
	index := 0

	for !finished {
		finished = true

		for _, solution := range solutions {
			if versions, ok := found[solution.Name()]; ok && index < len(versions) {
				finished = false

				sorted = append(sorted, versions[index])
			}
		}

		index++
	}

	if len(sorted) == 0 {
		return nil, errors.Wrap(
			domain.ErrImageNotFound,
			errors.WithDetail("image not found in filesystem registry"),
			ocierrors.BuildOCIProperties(
				ocierrors.NameUnknown,
				domain.ErrImageNotFound.Error(),
				map[string]string{"image_name": string(imageName)},
			),
		)
	}

	return sorted, nil
}

// isLayout reports whether base is an OCI Image Layout (has oci-layout and
// index.json). Stray directories that are not layouts are skipped.
func (fs *FileSystem) isLayout(ctx context.Context, l *slog.Logger, base string) bool {
	for _, marker := range []string{"oci-layout", "index.json"} {
		info, err := fs.root.Stat(base + "/" + marker)
		if err != nil || info.IsDir() {
			l.WarnContext(ctx, "image directory is not an OCI layout, skipping",
				slog.String("path", base), slog.String("missing", marker))

			return false
		}
	}

	return true
}
