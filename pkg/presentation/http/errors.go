package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
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
func HandleError(ctx context.Context, w http.ResponseWriter, err error, l *slog.Logger) {
	l.WarnContext(ctx, "handling error", slog.Any("error", err))

	if errors.Is(err, domain.ErrRegistryInternal) {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if ociErr, ok := ocierrors.AsOCIError(err); ok {
		errorResponse := NewErrorResponse()
		errorResponse.AddError(*ociErr)
		RespondWithJSON(ctx, w, errorResponse, nil, http.StatusNotFound, l)

		return
	}

	l.WarnContext(ctx, "is an unhandled error type", slog.Any("error", err))
	http.Error(w, "", http.StatusNotFound)
}
