package checker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
)

type All []Interface

func (a All) Check(r *http.Request) (bool, error) {
	for _, c := range a {
		match, err := c.Check(r)
		if err != nil {
			return match, err
		}
		if !match {
			return false, err // no match
		}
	}

	return true, nil // match
}

func (a All) Hash() string {
	var sb strings.Builder

	for _, c := range a {
		fmt.Fprintln(&sb, c.Hash())
	}

	return internal.FastHash(sb.String())
}
