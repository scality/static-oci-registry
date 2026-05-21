package http

import (
	"log/slog"
	"net/http"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	apperrors "github.com/scality/static-oci-registry/pkg/errors"

	"github.com/pkg/errors"
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
func HandleError(w http.ResponseWriter, err error, l *slog.Logger) {
	l.Warn("handling error", slog.Any("error_message", err))

	if errors.Is(errors.Cause(err), domain.ErrRegistryInternal) {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if ociErr, ok := apperrors.AsOCIError(err); ok {
		errorResponse := NewErrorResponse()
		errorResponse.AddError(*ociErr)
		RespondWithJSON(w, errorResponse, http.StatusNotFound, l)

		return
	}

	l.Warn("is an unhandled error type", slog.Any("error_message", err))
	http.Error(w, "", http.StatusNotFound)
}
