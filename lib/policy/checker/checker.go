// Package checker defines the Checker interface and a helper utility to avoid import cycles.
package checker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
)

type Impl interface {
	Check(*http.Request) (bool, error)
	Hash() string
}

type List []Impl

// Check runs each checker in the list against the request.
// It returns true only if *all* checkers return true (AND semantics).
// If any checker returns an error, the function returns false and the error.
func (l List) Check(r *http.Request) (bool, error) {
	// Assume success until a checker says otherwise.
	allOk := true
	for _, c := range l {
		ok, err := c.Check(r)
		if err != nil {
			// Propagate the error; overall result is false.
			return false, err
		}
		if !ok {
			// One false means the combined result is false. Short-circuit
			// so we don't waste time.
			allOk = false
			break
		}
	}
	return allOk, nil
}

func (l List) Hash() string {
	var sb strings.Builder

	for _, c := range l {
		fmt.Fprintln(&sb, c.Hash())
	}

	return internal.FastHash(sb.String())
}
