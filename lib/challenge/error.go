package challenge

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrFailed        = errors.New("challenge: user failed challenge")
	ErrMissingField  = errors.New("challenge: missing field")
	ErrInvalidFormat = errors.New("challenge: field has invalid format")
)

func NewError(verb, publicReason string, privateReason error) *Error {
	return &Error{
		Verb:          verb,
		PublicReason:  publicReason,
		PrivateReason: privateReason,
		StatusCode:    http.StatusForbidden,
	}
}

type Error struct {
	PrivateReason error
	Verb          string
	PublicReason  string
	StatusCode    int
}

func (e *Error) Error() string {
	return fmt.Sprintf("challenge: error when processing challenge: %s: %v", e.Verb, e.PrivateReason)
}

func (e *Error) Unwrap() error {
	return e.PrivateReason
}
