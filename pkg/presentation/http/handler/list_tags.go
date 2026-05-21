package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/errors"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"
)

const (
	listTagsPrefix = "/v2/"
	listTagsSuffix = "/tags/list"
)

type ListTags struct {
	logger          *slog.Logger
	listTagsUseCase *usecase.ListTags
}

func NewListTags(
	listTagsUseCase *usecase.ListTags,
	logger *slog.Logger,
) *ListTags {
	return &ListTags{
		listTagsUseCase: listTagsUseCase,
		logger:          logger.With(slog.String("http_handler", "list_tags")),
	}
}

func (h *ListTags) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listTagsInput, err := parseRequest(r)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	listTagsOutput, err := h.listTagsUseCase.Execute(ctx, *listTagsInput)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	httplayer.RespondWithJSON(ctx, w, listTagsOutput, http.StatusOK, h.logger)
}

// nolint:funlen,gocognit // this function is long and complex because of all the
// checks and path parsing logic, and cannot be meaningfully shortened or split
// parses a query and return the input type for ListTags usecase.
func parseRequest(r *http.Request) (*domain.ListTagsInput, *errors.Error) {
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
		return nil, errors.FromCode(err, ocierrors.NameInvalid).
			Wrap("error validating image name in query parser").
			WithOCIMessage(err.Error()).
			WithOCIDetail("image_name", string(img))
	}

	listTagsInput := &domain.ListTagsInput{Name: img}

	q := r.URL.Query()

	last := q.Get("last")
	if last != "" {
		lastTag := domain.Tag(last)

		err := lastTag.Validate()
		if err != nil {
			return nil, errors.FromCode(err, ocierrors.Unsupported).
				Wrap("error validating last tag in query parser").
				WithOCIMessage(err.Error()).
				WithOCIDetail("last", last)
		}

		listTagsInput.Last = &lastTag
	}

	n := q.Get("n")
	if n != "" {
		nint, err := strconv.ParseInt(n, 0, 0)
		if err != nil {
			return nil, errors.FromCode(domain.ErrInvalidParameter, ocierrors.Unsupported).
				WrapErr(err).
				Wrap("error validating n parameter in query parser").
				WithOCIMessage("Invalid integer value in n parameter").
				WithOCIDetail("n", n)
		}

		tagLimit := int(nint)

		if tagLimit < 0 || tagLimit > 1000 {
			return nil, errors.FromCode(domain.ErrInvalidParameter, ocierrors.Unsupported).
				Wrap("n parameter is out of range in query parser").
				WithOCIMessage("n parameter must be between 0 and 1000").
				WithOCIDetail("n", strconv.Itoa(tagLimit))
		}

		listTagsInput.N = &tagLimit
	}

	return listTagsInput, nil
}
