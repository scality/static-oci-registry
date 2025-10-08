package handler

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/pkg/errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	httplayer "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/usecase"

	"github.com/rs/zerolog"
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
	img := domain.ImageName(r.PathValue("image"))

	err := img.Validate()
	if err != nil {
		httplayer.HandleError(w, err, img, h.logger)
		return
	}

	listTagsOutput, err := h.listTagsUseCase.Execute(img)
	if err != nil {
		httplayer.HandleError(w, err, img, h.logger)
		return
	}

	q := r.URL.Query()

	last := q.Get("last")
	if last != "" {
		lastTag := domain.Tag(last)

		err := lastTag.Validate()
		if err != nil {
			httplayer.HandleError(w, err, lastTag, h.logger)
			return
		}

		if !slices.Contains(listTagsOutput.Tags, lastTag) {
			httplayer.HandleError(w, domain.ErrTagNotFound, lastTag, h.logger)
			return
		}

		// return tags after lastTag without lastTag
		i := slices.Index(listTagsOutput.Tags, lastTag)
		listTagsOutput.Tags = listTagsOutput.Tags[i+1:]
	}

	n := q.Get("n")
	if n != "" {
		tagLimit, err := strconv.ParseInt(n, 0, 0)
		if err != nil {
			httplayer.HandleError(w, errors.Wrapf(domain.ErrInvalidParameter, "%s", err),
				n, h.logger)

			return
		}

		if tagLimit < 0 || tagLimit > 1000 {
			httplayer.HandleError(w, domain.ErrInvalidParameter, n, h.logger)
			return
		}

		if int(tagLimit) < len(listTagsOutput.Tags) {
			listTagsOutput.Tags = listTagsOutput.Tags[:tagLimit]
		}
	}

	httplayer.RespondWithJSON(w, listTagsOutput, http.StatusOK, h.logger)
}
