package checker

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
)

type Any []Interface

func (a Any) Check(r *http.Request) (bool, error) {
	for _, c := range a {
		match, err := c.Check(r)
		if err != nil {
			return match, err
		}
		if match {
			return true, err // match
		}
	}

	return false, nil // no match
}

func (a Any) Hash() string {
	var sb strings.Builder

	for _, c := range a {
		fmt.Fprintln(&sb, c.Hash())
	}

	return internal.FastHash(sb.String())
}
