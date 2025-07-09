package apperrors

import "net/http"

func BadRequest(message string, err ...error) error {
	return newError(http.StatusBadRequest, message, err...)
}

func Unauthorized(message string, err ...error) error {
	return newError(http.StatusUnauthorized, message, err...)
}

func PaymentRequired(message string, err ...error) error {
	return newError(http.StatusPaymentRequired, message, err...)
}

const (
	StatusLoginTimeout = 440
)

func Forbidden(message string, err ...error) error {
	return newError(http.StatusForbidden, message, err...)
}

func NotFound(message string, err ...error) error {
	return newError(http.StatusNotFound, message, err...)
}

func AlreadyExist(message string, err ...error) error {
	return newError(http.StatusConflict, message, err...)
}

// Teapot indicates that handler is a teapot and unable to brew a coffee
func Teapot(message string, err ...error) error {
	return newError(http.StatusTeapot, message, err...)
}

func LoginTimeout(message string, err ...error) error {
	return newError(StatusLoginTimeout, message, err...)
}

func Internal(message string, err ...error) error {
	return newError(http.StatusInternalServerError, message, err...)
}

func ServiceUnavailable(message string, err ...error) error {
	return newError(http.StatusServiceUnavailable, message, err...)
}
