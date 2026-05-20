package http

import (
	"net/http"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"

	"github.com/rs/zerolog"
)

type ErrorResponse struct {
	Errors []ocierrors.OCIError `json:"errors"`
}

func NewErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Errors: make([]ocierrors.OCIError, 0),
	}
}

func (er *ErrorResponse) AddError(oci ocierrors.OCIError) {
	er.Errors = append(er.Errors, oci)
}

// nolint:funlen // this contains a long, unsplittable switch statement
// HandleError handles domain errors and sends appropriate HTTP responses based on the error type.
func HandleError(w http.ResponseWriter, err error, l *zerolog.Logger) {
	l.Warn().Err(err).Msg("handling error")

	if errors.Is(err, domain.ErrRegistryInternal) {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if ociErr, ok := ocierrors.AsOCIError(err); ok {
		errorResponse := NewErrorResponse()
		errorResponse.AddError(*ociErr)
		RespondWithJSON(w, errorResponse, http.StatusNotFound, l)

		return
	}

	l.Warn().Err(err).Msg("is an unhandled error type")
	http.Error(w, "", http.StatusNotFound)
}
