package domain

import "fmt"

type Error struct {
	Code    string
	Message string
	Field   string
}

func (e *Error) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func NewError(code, message, field string) error {
	return &Error{Code: code, Message: message, Field: field}
}

func Required(field string) error {
	return NewError("validation_error", "不能为空", field)
}

func Invalid(field, message string) error {
	return NewError("validation_error", message, field)
}

func Conflict(message string) error {
	return NewError("conflict", message, "")
}

func NotFound(entity string) error {
	return NewError("not_found", entity+"不存在", "")
}

func StateConflict(message string) error {
	return NewError("invalid_state", message, "")
}
