package apperrors

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/goccy/go-json"
)

type Error struct {
	BaseError error
	Message   string
	Status    int
	path      string
}

func newError(httpStatus int, message string, err ...error) *Error {
	var baseError error
	if len(err) == 0 {
		baseError = nil
	} else {
		baseError = err[0]
	}

	var e *Error
	ok := errors.As(baseError, &e)
	if ok {
		return e.wrap(message, httpStatus)
	}

	var path string
	if _, fileErrOccurred, lineErrOccurred, ok := runtime.Caller(2); ok {
		path = fmt.Sprintf("%s:%d", fileErrOccurred, lineErrOccurred)
	}

	appErr := &Error{
		BaseError: baseError,
		Message:   message,
		Status:    httpStatus,
		path:      path,
	}

	return appErr
}

type ErrorPublic struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	b, _ := json.Marshal(
		&ErrorPublic{
			Message: e.Message,
			Status:  e.Status,
		},
	)

	return string(b)
}

func (e *Error) wrap(message string, httpStatus int) *Error {
	if e == nil {
		return newError(httpStatus, message)
	}

	return &Error{
		BaseError: e.BaseError,
		Message:   fmt.Sprintf("%s: %s", message, e.Message),
		Status:    httpStatus,
		path:      e.path,
	}
}

func (e *Error) Path() string {
	return e.path
}

func IsAppError(err error) (*Error, bool) {
	var appErr *Error
	ok := errors.As(err, &appErr)

	return appErr, ok
}
