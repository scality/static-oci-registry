package handler

import (
	"net/http"
	"strconv"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"

	"github.com/rs/zerolog"
)

const (
	listTagsPrefix = "/v2/"
	listTagsSuffix = "/tags/list"
)

type ListTags struct {
	logger          *zerolog.Logger
	listTagsUseCase *usecase.ListTags
}

func NewListTags(
	listTagsUseCase *usecase.ListTags,
	logger *zerolog.Logger,
) *ListTags {
	l := logger.With().Str("http_handler", "list_tags").Logger()

	return &ListTags{
		listTagsUseCase: listTagsUseCase,
		logger:          &l,
	}
}

func (h *ListTags) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	listTagsInput, err := parseRequest(r)
	if err != nil {
		httplayer.HandleError(w, err, h.logger)
		return
	}

	listTagsOutput, err := h.listTagsUseCase.Execute(*listTagsInput)
	if err != nil {
		httplayer.HandleError(w, err, h.logger)
		return
	}

	httplayer.RespondWithJSON(w, listTagsOutput, http.StatusOK, h.logger)
}

// nolint:funlen,gocognit // this function is long and complex because of all the
// checks and path parsing logic, and cannot be meaningfully shortened or split
// parses a query and return the input type for ListTags usecase.
func parseRequest(r *http.Request) (*domain.ListTagsInput, error) {
	// Extract image name from URL path
	// Path format: /v2/{image}/tags/list
	// We use manual path parsing to support multi-level image names with slashes
	path := r.URL.Path

	img := domain.ImageName("")
	if len(path) > len(listTagsPrefix)+len(listTagsSuffix) {
		img = domain.ImageName(path[len(listTagsPrefix) : len(path)-len(listTagsSuffix)])
	}

	err := img.Validate()
	if err != nil {
		// this is a domain error so no need to redefine it here
		return nil, errors.Wrap(
			err,
			errors.WithDetail("error validating image name in query parser"),
			errors.WithProperty(ocierrors.OCICode, ocierrors.NameInvalid),
			errors.WithProperty(ocierrors.OCIMessage, err.Error()),
			errors.WithProperty(ocierrors.OCIPrefix+"image_name", string(img)),
		)
	}

	listTagsInput := &domain.ListTagsInput{Name: img}

	q := r.URL.Query()

	last := q.Get("last")
	if last != "" {
		lastTag := domain.Tag(last)

		err := lastTag.Validate()
		if err != nil {
			return nil, errors.Wrap(
				err,
				errors.WithDetail("error validating last tag in query parser"),
				errors.WithProperty(ocierrors.OCICode, ocierrors.Unsupported),
				errors.WithProperty(ocierrors.OCIMessage, err.Error()),
				errors.WithProperty(ocierrors.OCIPrefix+"last", last),
			)
		}

		listTagsInput.Last = &lastTag
	}

	n := q.Get("n")
	if n != "" {
		nint, err := strconv.ParseInt(n, 0, 0)
		if err != nil {
			return nil, errors.Wrap(
				domain.ErrInvalidParameter,
				errors.WithDetail("error validating n parameter in query parser"),
				errors.WithProperty(ocierrors.OCICode, ocierrors.Unsupported),
				errors.WithProperty(ocierrors.OCIMessage, "Invalid integer value in n parameter"),
				errors.WithProperty(ocierrors.OCIPrefix+"n", n),
				errors.CausedBy(err),
			)
		}

		tagLimit := int(nint)

		if tagLimit < 0 || tagLimit > 1000 {
			return nil, errors.Wrap(
				domain.ErrInvalidParameter,
				errors.WithDetail("n parameter is out of range in query parser"),
				errors.WithProperty(ocierrors.OCICode, ocierrors.Unsupported),
				errors.WithProperty(ocierrors.OCIMessage, "n parameter must be between 0 and 1000"),
				errors.WithProperty(ocierrors.OCIPrefix+"n", strconv.Itoa(tagLimit)),
			)
		}

		listTagsInput.N = &tagLimit
	}

	return listTagsInput, nil
}
