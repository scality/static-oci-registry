package ocierrors

import (
	"strings"

	"github.com/scality/go-errors"
)

// these codes are defined in the OCI Distribution Spec
// https://github.com/opencontainers/distribution-spec/blob/v1.0.1/spec.md#error-codes
type OCIErrorCode string

type OCIError struct {
	Code    OCIErrorCode      `json:"code"`
	Message string            `json:"message,omitempty"`
	Detail  map[string]string `json:"detail,omitempty"`
}

const (
	BlobUnknown OCIErrorCode = "BLOB_UNKNOWN"
	// BlobUploadInvalid    OCIErrorCode = "BLOB_UPLOAD_INVALID"
	// BlobUploadUnknown    OCIErrorCode = "BLOB_UPLOAD_UNKNOWN".
	DigestInvalid OCIErrorCode = "DIGEST_INVALID"
	// ManifestBlobUnknown  OCIErrorCode = "MANIFEST_BLOB_UNKNOWN"
	// ManifestInvalid      OCIErrorCode = "MANIFEST_INVALID".
	ManifestUnknown OCIErrorCode = "MANIFEST_UNKNOWN"
	NameInvalid     OCIErrorCode = "NAME_INVALID"
	NameUnknown     OCIErrorCode = "NAME_UNKNOWN"
	// SizeInvalid         OCIErrorCode = "SIZE_INVALID"
	// Unauthorized        OCIErrorCode = "UNAUTHORIZED"
	// Denied              OCIErrorCode = "DENIED".
	Unsupported OCIErrorCode = "UNSUPPORTED"
	// TooManyRequests     OCIErrorCode = "TOO_MANY_REQUESTS".

	// constants for OCI_xxx properties.
	OCIPrefix  string = "OCI_"
	OCICode    string = OCIPrefix + "CODE"
	OCIMessage string = OCIPrefix + "MESSAGE"
)

func AsOCIError(err error) (*OCIError, bool) {
	var e *errors.Error
	if !errors.As(err, &e) {
		return nil, false
	}

	code, message, details := extractOCIProps(e.Properties)
	if code == "" {
		return nil, false
	}

	return &OCIError{Code: code, Message: message, Detail: details}, true
}

func BuildOCIProperties(code OCIErrorCode, msg string, details map[string]string) errors.Option {
	prop := make(map[string]any)

	prop[OCICode] = code
	if msg != "" {
		prop[OCIMessage] = msg
	}

	for k, v := range details {
		prop[OCIPrefix+k] = v
	}

	return errors.WithProperties(prop)
}

func extractCodeAndMessage(
	k string, v any, code OCIErrorCode, message string,
) (OCIErrorCode, string) {
	switch k {
	case OCICode:
		if val, ok := v.(OCIErrorCode); ok {
			return val, message
		}
	case OCIMessage:
		if val, ok := v.(string); ok {
			return code, val
		}
	}

	return code, message
}

func extractOCIProps(props map[string]any) (OCIErrorCode, string, map[string]string) {
	var code OCIErrorCode

	message := ""
	details := make(map[string]string)

	for k, v := range props {
		code, message = extractCodeAndMessage(k, v, code, message)

		if key, found := strings.CutPrefix(k, OCIPrefix); found {
			if val, ok := v.(string); ok {
				details[key] = val
			}
		}
	}

	return code, message, details
}
