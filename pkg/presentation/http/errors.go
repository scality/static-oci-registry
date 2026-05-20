package http

import (
	"net/http"
	"strings"

	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"

	"github.com/rs/zerolog"
)

type OCIError struct {
	Code    ocierrors.OCIErrorCode `json:"code"`
	Message string                 `json:"message,omitempty"`
	Detail  map[string]string      `json:"detail,omitempty"`
}

type ErrorResponse struct {
	Errors []OCIError `json:"errors"`
}

func NewErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Errors: make([]OCIError, 0),
	}
}

func (er *ErrorResponse) AddError(oci OCIError) {
	er.Errors = append(er.Errors, oci)
}

// nolint: gocognit,nestif // can't do anything about this function
func AsOCIError(err error) (*OCIError, bool) {
	var e *errors.Error
	if errors.As(err, &e) {
		// let's build a proper OCIError from the properties of the error
		var code ocierrors.OCIErrorCode

		message := ""
		details := make(map[string]string)

		for k, v := range e.Properties {
			if k == ocierrors.OCICode {
				if val, ok := v.(ocierrors.OCIErrorCode); ok {
					code = val
				}

				continue
			}

			if k == ocierrors.OCIMessage {
				if val, ok := v.(string); ok {
					message = val
				}

				continue
			}

			if key, ok := strings.CutPrefix(k, ocierrors.OCIPrefix); ok {
				if val, ok := v.(string); ok {
					// remove the prefix and add to details
					details[key] = val
				}
			}
		}

		if code != "" {
			return &OCIError{Code: code, Message: message, Detail: details}, true
		}
	}

	return nil, false
}

// nolint:funlen // this contains a long, unsplittable switch statement
// HandleError handles domain errors and sends appropriate HTTP responses based on the error type.
func HandleError(w http.ResponseWriter, err error, l *zerolog.Logger) {
	l.Warn().Err(err).Msg("handling error")

	if errors.Is(err, domain.ErrRegistryInternal) {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if ociErr, ok := AsOCIError(err); ok {
		errorResponse := NewErrorResponse()
		errorResponse.AddError(*ociErr)
		RespondWithJSON(w, errorResponse, http.StatusNotFound, l)

		return
	}

	l.Warn().Err(err).Msg("is an unhandled error type")
	http.Error(w, "", http.StatusNotFound)
}
