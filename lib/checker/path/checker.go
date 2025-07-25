package path

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
)

func New(rexStr string) (checker.Interface, error) {
	rex, err := regexp.Compile(strings.TrimSpace(rexStr))
	if err != nil {
		return nil, fmt.Errorf("%w: regex %s failed parse: %w", anubis.ErrMisconfiguration, rexStr, err)
	}
	return &Checker{rex, internal.FastHash(rexStr)}, nil
}

type Checker struct {
	regexp *regexp.Regexp
	hash   string
}

func (c *Checker) Check(r *http.Request) (bool, error) {
	if c.regexp.MatchString(r.URL.Path) {
		return true, nil
	}

	return false, nil
}

func (c *Checker) Hash() string {
	return c.hash
}
