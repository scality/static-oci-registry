package errors

import (
	"github.com/pkg/errors"

	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
)

type Error struct {
	cause    error
	ociError *ocierrors.OCIError
}

func (e Error) Error() string {
	return e.cause.Error()
}

func (e *Error) Unwrap() error {
	return e.cause
}

func FromCode(cause error, code ocierrors.OCIErrorCode) *Error {
	return &Error{cause: cause, ociError: &ocierrors.OCIError{Code: code}}
}

func New(cause error, oci *ocierrors.OCIError) *Error {
	return &Error{cause: cause, ociError: oci}
}

func (e *Error) Wrap(msg string) *Error {
	return New(errors.Wrap(e.cause, msg), e.ociError)
}

func (e *Error) Wrapf(format string, args ...any) *Error {
	return New(errors.Wrapf(e.cause, format, args...), e.ociError)
}

func (e *Error) WrapErr(cause error) *Error {
	return e.Wrapf("caused by: %v", cause)
}

func AsOCIError(err error) (*ocierrors.OCIError, bool) {
	var w *Error
	if errors.As(err, &w) {
		return w.ociError, true
	}

	return nil, false
}

func (e *Error) WithOCIDetail(key, value string) *Error {
	if e.ociError != nil {
		return New(e.cause, e.ociError.WithDetail(key, value))
	}

	return e
}

func (e *Error) WithOCIMessage(msg string) *Error {
	if e.ociError != nil {
		return New(e, e.ociError.WithMessage(msg))
	}

	return e
}
