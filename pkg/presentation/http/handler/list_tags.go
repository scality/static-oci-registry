package handler

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"
)

const listTagsURLPattern = `^/v2/(.+)/tags/list$`

var listTagsURLRegex = regexp.MustCompile(listTagsURLPattern)

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

// Matches reports whether the given request path is served by this handler.
func (*ListTags) Matches(path string) bool {
	return listTagsURLRegex.MatchString(path)
}

func (h *ListTags) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listTagsInput, err := parseListTagRequest(r)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	listTagsOutput, err := h.listTagsUseCase.Execute(ctx, *listTagsInput)
	if err != nil {
		httplayer.HandleError(ctx, w, err, h.logger)
		return
	}

	httplayer.RespondWithJSON(ctx, w, listTagsOutput, nil, http.StatusOK, h.logger)
}

// nolint:funlen,gocognit // this function is long and complex because of all the
// checks and path parsing logic, and cannot be meaningfully shortened or split
// parses a query and return the input type for ListTags usecase.
func parseListTagRequest(r *http.Request) (*domain.ListTagsInput, error) {
	// Extract image name from URL path
	// Path format: /v2/{image}/tags/list
	// use a regular expression to extract the image name
	// since the image name can contain multiple levels of slashes
	path := r.URL.Path

	matches := listTagsURLRegex.FindStringSubmatch(path)
	if matches == nil {
		return nil, errors.Wrap(
			domain.ErrInvalidRequest,
			errors.WithDetail("error parsing URL path in query parser"),
			ocierrors.BuildOCIProperties(
				ocierrors.Unsupported,
				domain.ErrInvalidRequest.Error(),
				map[string]string{"path": path},
			),
		)
	}

	img := domain.ImageName(matches[1])

	err := img.Validate()
	if err != nil {
		// this is a domain error so no need to redefine it here
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
				ocierrors.BuildOCIProperties(
					ocierrors.Unsupported,
					err.Error(),
					map[string]string{"last": last},
				),
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
				ocierrors.BuildOCIProperties(
					ocierrors.Unsupported,
					"Invalid integer value in n parameter",
					map[string]string{"n": n},
				),
				errors.CausedBy(err),
			)
		}

		tagLimit := int(nint)

		if tagLimit < 0 || tagLimit > 1000 {
			return nil, errors.Wrap(
				domain.ErrInvalidParameter,
				errors.WithDetail("n parameter is out of range in query parser"),
				ocierrors.BuildOCIProperties(
					ocierrors.Unsupported,
					"n parameter must be between 0 and 1000",
					map[string]string{"n": strconv.Itoa(tagLimit)},
				),
			)
		}

		listTagsInput.N = &tagLimit
	}

	return listTagsInput, nil
}
