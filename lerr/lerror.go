package lerr

import (
	"errors"
	"fmt"
	"net/http"
)

// UnknownErrorCode is the default error code for errors where the code is not set or known.
var UnknownErrorCode = http.StatusInternalServerError

// DetailedError is a custom error type for detailed error handling in http requests.
type DetailedError struct {
	err  error
	code int
	// A user-friendly description of the error that can be presented to the user.
	// description string
	// details     map[string]any
}

// New creates a new DetailedError with the given message and code.
func New(code int, message string) *DetailedError {
	return &DetailedError{
		err:  errors.New(message),
		code: code,
	}
}

// Newf creates a new DetailedError with the given formatted message and code.
// The format string and args are passed to fmt.Errorf.
func Newf(code int, format string, a ...any) *DetailedError {
	return &DetailedError{
		err:  fmt.Errorf(format, a...),
		code: code,
	}
}

// Wrap wraps the given error with a DetailedError with the given code and message.
func Wrap(err error, code int, message string) *DetailedError {
	return &DetailedError{
		err:  fmt.Errorf("%s: %w", message, err),
		code: code,
	}
}

// Code returns the code of the error if it implements the Code method.
// Otherwise returns the UnknownErrorCode.
func Code(err error) int {
	if e, ok := err.(interface{ Code() int }); ok { //nolint:errorlint // Only check top-level error.
		return e.Code()
	}
	return UnknownErrorCode
}

func (e *DetailedError) Error() string {
	return e.err.Error()
}

func (e *DetailedError) Code() int {
	if e.code == 0 {
		return UnknownErrorCode
	}

	return e.code
}

func (e *DetailedError) Unwrap() error {
	return e.err
}

// func (e *DetailedError) WithDescription(description string) {
// 	e.description = description
// }

// func (e *DetailedError) Description() string {
// 	return e.description
// }

// func (e *DetailedError) AddDetail(key string, value any) {
// 	e.details[key] = value
// }

// func (e *DetailedError) Details() map[string]any {
// 	return e.details
// }
