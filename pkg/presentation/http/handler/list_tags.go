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

// AllowedMethods reports the HTTP methods this endpoint accepts.
// end-8 of the OCI distribution-spec is GET-only.
func (*ListTags) AllowedMethods() []string {
	return []string{http.MethodGet}
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
		// Unreachable: the router only dispatches to this handler when
		// ListTags.Matches(path) returns true, which uses the same regex.
		// A nil result here means the router and handler are out of sync
		// — a programming error, not a client error.
		panic("list_tags: router/handler regex mismatch for path " + path)
	}

	img := domain.ImageName(parseNamespace(r) + matches[1])

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

	// Bad `last`/`n` query parameters are silently ignored, matching the
	// behavior of real registries (Quay, MCR) which return the full sorted
	// list instead of erroring on malformed pagination input.
	last := q.Get("last")
	if last != "" {
		lastTag := domain.Tag(last)
		if err := lastTag.Validate(); err == nil {
			listTagsInput.Last = &lastTag
		}
	}

	n := q.Get("n")
	if n != "" {
		if nint, err := strconv.ParseInt(n, 0, 0); err == nil {
			tagLimit := int(nint)
			if tagLimit >= 0 && tagLimit <= 1000 {
				listTagsInput.N = &tagLimit
			}
		}
	}

	return listTagsInput, nil
}
