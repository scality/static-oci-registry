package handler

import (
	"log/slog"
	"net/http"
	"regexp"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"
)

const fetchManifestURLPattern = `^/v2/(.+)/manifests/([^/]+)$`

var fetchManifestURLRegex = regexp.MustCompile(fetchManifestURLPattern)

// Matches reports whether the given request path is served by this handler.
func (*FetchManifest) Matches(path string) bool {
	return fetchManifestURLRegex.MatchString(path)
}

type FetchManifestInput struct {
	Name domain.ImageName
	Ref  domain.ManifestReference
	Head bool
}

type FetchManifest struct {
	logger                         *slog.Logger
	fetchManifestFromTagUseCase    *usecase.FetchManifestFromTag
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest
}

func NewFetchManifest(
	logger *slog.Logger,
	fetchManifestFromTagUseCase *usecase.FetchManifestFromTag,
	fetchManifestFromDigestUseCase *usecase.FetchManifestFromDigest,
) *FetchManifest {
	return &FetchManifest{
		logger:                         logger,
		fetchManifestFromTagUseCase:    fetchManifestFromTagUseCase,
		fetchManifestFromDigestUseCase: fetchManifestFromDigestUseCase,
	}
}

func (h *FetchManifest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fetchManifestInput, err := parseFetchManifestRequest(r)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	var fetchManifestOutput *domain.FetchManifestOutput

	switch ref := fetchManifestInput.Ref.(type) {
	case domain.Tag:
		fetchManifestOutput, err = h.fetchManifestFromTagUseCase.Execute(
			ctx, fetchManifestInput.Name, ref)
	case domain.Digest:
		fetchManifestOutput, err = h.fetchManifestFromDigestUseCase.Execute(
			ctx, fetchManifestInput.Name, ref)
	default:
		// this should never happen
		httplayer.HandleError(ctx, w, domain.ErrRegistryInternal, h.logger)
		return
	}

	// failure in a usecase
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	// respond with json
	// must set Content-Type to mediaType and
	// must contain digest from manifest in docker-content-digest header
	headers := map[string]string{
		"Content-Type":          fetchManifestOutput.MediaType,
		"Docker-Content-Digest": fetchManifestOutput.ContentDigest.String(),
	}

	if fetchManifestInput.Head {
		for key, value := range headers {
			w.Header().Set(key, value)
		}

		w.WriteHeader(http.StatusOK)

		return
	}

	httplayer.RespondWithBytes(
		ctx, w, fetchManifestOutput.ManifestBytes, headers, http.StatusOK, h.logger)
}

// nolint:funlen,gocognit // this function is long and complex because of all the
// checks and path parsing logic, and cannot be meaningfully shortened or split
// parses a query and return the input type for FetchManifest usecase.
func parseFetchManifestRequest(r *http.Request) (*FetchManifestInput, error) {
	// Extract image name and ref from URL path
	// Path format: /v2/{image}/manifests/{reference}
	// use a regular expression to extract the image name and reference
	// since the image name can contain multiple levels of slashes
	path := r.URL.Path

	matches := fetchManifestURLRegex.FindStringSubmatch(path)
	if matches == nil {
		// Unreachable: the router only dispatches to this handler when
		// FetchManifest.Matches(path) returns true, which uses the same
		// regex. A nil result here means the router and handler are out
		// of sync — a programming error, not a client error.
		panic("fetch_manifest: router/handler regex mismatch for path " + path)
	}

	img := domain.ImageName(matches[1])
	if err := img.Validate(); err != nil {
		return nil, errors.Wrap(
			err,
			errors.WithDetail("error validating image name in query parser"),
			ocierrors.BuildOCIProperties(
				ocierrors.NameInvalid,
				err.Error(),
				map[string]string{"image_name": string(img)},
			),
		)
	}

	var ref domain.ManifestReference

	// we have to try both types here, start with hash since it's more specific
	digest := domain.Digest(matches[2])
	if err := digest.Validate(); err == nil {
		ref = digest
	}

	// then we check if it's a tag
	if ref == nil {
		tag := domain.Tag(matches[2])
		if err := tag.Validate(); err == nil {
			ref = tag
		}
	}

	if ref == nil {
		return nil, errors.Wrap(
			domain.ErrInvalidReference,
			errors.WithDetail("error validating manifest reference in query parser"),
			ocierrors.BuildOCIProperties(
				ocierrors.ManifestUnknown,
				domain.ErrInvalidReference.Error(),
				map[string]string{"reference": matches[2]},
			),
		)
	}

	return &FetchManifestInput{
		Name: img,
		Ref:  ref,
		Head: r.Method == http.MethodHead,
	}, nil
}
