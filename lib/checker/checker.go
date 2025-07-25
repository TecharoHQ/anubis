// Package checker defines the Checker interface and a helper utility to avoid import cycles.
package checker

import (
	"errors"
	"net/http"
)

var (
	ErrUnparseableConfig = errors.New("checker: config is unparseable")
	ErrInvalidConfig     = errors.New("checker: config is invalid")
)

type Interface interface {
	Check(*http.Request) (matches bool, err error)
	Hash() string
}
