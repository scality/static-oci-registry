package ocierrors

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

	// constants for OCI_xxx properties.
	OCIPrefix  string = "OCI_"
	OCICode    string = OCIPrefix + "CODE"
	OCIMessage string = OCIPrefix + "MESSAGE"
)
