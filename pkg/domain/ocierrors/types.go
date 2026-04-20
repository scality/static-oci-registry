package ocierrors

import (
	"maps"
)

// these codes are defined in the OCI Distribution Spec
// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#error-codes
type OCIErrorCode string

const (
	// BlobUnknown          OCIErrorCode = "BLOB_UNKNOWN"
	// BlobUploadInvalid    OCIErrorCode = "BLOB_UPLOAD_INVALID"
	// BlobUploadUnknown    OCIErrorCode = "BLOB_UPLOAD_UNKNOWN"
	// DigestInvalid        OCIErrorCode = "DIGEST_INVALID"
	// ManifestBlobUnknown  OCIErrorCode = "MANIFEST_BLOB_UNKNOWN"
	// ManifestInvalid      OCIErrorCode = "MANIFEST_INVALID"
	// ManifestUnknown      OCIErrorCode = "MANIFEST_UNKNOWN".
	NameInvalid OCIErrorCode = "NAME_INVALID"
	NameUnknown OCIErrorCode = "NAME_UNKNOWN"
	// SizeInvalid         OCIErrorCode = "SIZE_INVALID"
	// Unauthorized        OCIErrorCode = "UNAUTHORIZED"
	// Denied              OCIErrorCode = "DENIED".
	Unsupported OCIErrorCode = "UNSUPPORTED"
	// TooManyRequests     OCIErrorCode = "TOO_MANY_REQUESTS".
)

// this structure is defined in the OCI distribution Spec
// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#error-codes
type OCIError struct {
	Code    OCIErrorCode      `json:"code"`
	Message string            `json:"message,omitempty"`
	Detail  map[string]string `json:"detail,omitempty"`
}

func (e *OCIError) WithDetail(key, value string) *OCIError {
	ret := &OCIError{
		Code:    e.Code,
		Message: e.Message,
	}
	if e.Detail != nil {
		ret.Detail = make(map[string]string, len(e.Detail)+1)
		maps.Copy(ret.Detail, e.Detail)
	} else {
		ret.Detail = make(map[string]string, 1)
	}

	ret.Detail[key] = value

	return ret
}

func (e *OCIError) WithMessage(msg string) *OCIError {
	return &OCIError{
		Code:    e.Code,
		Message: msg,
		Detail:  e.Detail,
	}
}

func NewNameInvalid() *OCIError {
	return &OCIError{Code: NameInvalid}
}

func NewNameUnknown() *OCIError {
	return &OCIError{Code: NameUnknown}
}

func NewUnsupported() *OCIError {
	return &OCIError{Code: Unsupported}
}
