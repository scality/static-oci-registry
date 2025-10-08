package http

import (
	"fmt"
	"net/http"

	"github.com/scality/static-oci-registry/pkg/domain"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// these codes are defined in the OCI Distribution Spec
// https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#error-codes
type OCIHTTPErrorCode string

const (
	// BlobUnknown          OCIHTTPErrorCode = "BLOB_UNKNOWN"
	// BlobUploadInvalid    OCIHTTPErrorCode = "BLOB_UPLOAD_INVALID"
	// BlobUploadUnknown    OCIHTTPErrorCode = "BLOB_UPLOAD_UNKNOWN"
	// DigestInvalid        OCIHTTPErrorCode = "DIGEST_INVALID"
	// ManifestBlobUnknown  OCIHTTPErrorCode = "MANIFEST_BLOB_UNKNOWN"
	// ManifestInvalid      OCIHTTPErrorCode = "MANIFEST_INVALID"
	// ManifestUnknown      OCIHTTPErrorCode = "MANIFEST_UNKNOWN".
	NameInvalid OCIHTTPErrorCode = "NAME_INVALID"
	NameUnknown OCIHTTPErrorCode = "NAME_UNKNOWN"
	// SizeInvalid         OCIHTTPErrorCode = "SIZE_INVALID"
	// Unauthorized        OCIHTTPErrorCode = "UNAUTHORIZED"
	// Denied              OCIHTTPErrorCode = "DENIED".
	Unsupported OCIHTTPErrorCode = "UNSUPPORTED"
	// TooManyRequests     OCIHTTPErrorCode = "TOO_MANY_REQUESTS".
)

type ErrorResponse struct {
	Errors []ErrorObject `json:"errors"`
}

type ErrorObject struct {
	Code    OCIHTTPErrorCode `json:"code"`
	Message string           `json:"message"`
	Detail  string           `json:"detail,omitempty"`
}

func NewErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Errors: make([]ErrorObject, 0),
	}
}

func (er *ErrorResponse) AddError(code OCIHTTPErrorCode, message, detail string) {
	er.Errors = append(er.Errors, ErrorObject{
		Code:    code,
		Message: message,
		Detail:  detail,
	})
}

// nolint:funlen // this contains a long, unsplittable switch statement
// HandleError handles domain errors and sends appropriate HTTP responses based on the error type.
func HandleError(w http.ResponseWriter, err error, data any, l *zerolog.Logger) {
	l.Warn().Err(err).Msg("handling error")

	var ociCode OCIHTTPErrorCode

	switch {
	case errors.Is(errors.Cause(err), domain.ErrInvalidImageName):
		ociCode = NameInvalid
	case errors.Is(errors.Cause(err), domain.ErrImageNotFound):
		ociCode = NameUnknown
	case errors.Is(errors.Cause(err), domain.ErrInvalidTag):
		ociCode = Unsupported
	case errors.Is(errors.Cause(err), domain.ErrTagNotFound):
		ociCode = Unsupported
	case errors.Is(errors.Cause(err), domain.ErrInvalidParameter):
		ociCode = Unsupported
	case errors.Is(errors.Cause(err), domain.ErrRegistryInternal):
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	default:
		l.Warn().Err(err).Msg("is an unhandled error type")
		http.Error(w, err.Error(), http.StatusNotFound)

		return
	}

	errorResponse := NewErrorResponse()

	// write data to detail if possible
	detail := ""

	if data != nil {
		if s, ok := data.(string); ok {
			detail = s
		} else {
			// TODO: reconsider this
			detail = fmt.Sprintf("%v", data)
		}
	}

	errorResponse.AddError(ociCode, errors.Cause(err).Error(), detail)

	RespondWithJSON(w, errorResponse, http.StatusNotFound, l)
}
