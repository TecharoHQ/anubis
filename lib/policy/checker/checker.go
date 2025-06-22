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

func (l List) Check(r *http.Request) (bool, error) {
	for _, c := range l {
		ok, err := c.Check(r)
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

func (l List) Hash() string {
	var sb strings.Builder

	for _, c := range l {
		fmt.Fprintln(&sb, c.Hash())
	}

	return internal.FastHash(sb.String())
}
