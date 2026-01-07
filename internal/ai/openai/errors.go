package openai

import "fmt"

type ErrorType int

const (
	ErrTypeAuth ErrorType = iota
	ErrTypeRateLimit
	ErrTypeInvalidRequest
	ErrTypeAPIError
	ErrTypeNetworkError
	ErrTypeSchemaValidation
	ErrTypeResponseParsing
)

type Error struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func isRetryable(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Type == ErrTypeRateLimit || e.Type == ErrTypeNetworkError
	}
	return false
}
