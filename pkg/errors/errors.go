package errors

import "net/http"

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code int, msg string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
		Err:     err,
	}
}

func InvalidRequest(err error) *AppError {
	return NewAppError(http.StatusBadRequest, "Invalid Request", err)
}

func NotFound(msg string) *AppError {
	return NewAppError(http.StatusNotFound, msg, nil)
}

func Unauthorized(msg string) *AppError {
	return NewAppError(http.StatusUnauthorized, msg, nil)
}

func InternalServerError() *AppError {
	return NewAppError(http.StatusInternalServerError, "Internal Server Error", nil)
}
