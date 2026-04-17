package coreerrors

import (
	"errors"
	"fmt"
)

type Category string
type Code string

const (
	CategoryValidation Category = "validation"
	CategorySecurity   Category = "security"
	CategoryExternal   Category = "external"
	CategoryParsing    Category = "parsing"
	CategoryStorage    Category = "storage"
	CategoryInternal   Category = "internal"
	CategoryConflict   Category = "conflict"
	CategoryNotFound   Category = "not_found"
)

const (
	CodeInvalidRequest      Code = "invalid_request"
	CodeUnauthorized        Code = "unauthorized"
	CodeProviderFailed      Code = "provider_failed"
	CodeUnexpectedStatus    Code = "unexpected_status"
	CodeParseFailed         Code = "parse_failed"
	CodeNormalizationFailed Code = "normalization_failed"
	CodeStorageFailed       Code = "storage_failed"
	CodeNotFound            Code = "not_found"
	CodeConflict            Code = "conflict"
	CodeInternal            Code = "internal_error"
)

type Error struct {
	Category  Category
	Code      Code
	Operation string
	Message   string
	Retryable bool
	Err       error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Operation == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.Operation, e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(category Category, code Code, operation string, retryable bool, message string, err error) *Error {
	return &Error{
		Category:  category,
		Code:      code,
		Operation: operation,
		Message:   message,
		Retryable: retryable,
		Err:       err,
	}
}

func IsCode(err error, code Code) bool {
	var typed *Error
	if !errors.As(err, &typed) {
		return false
	}
	return typed.Code == code
}
