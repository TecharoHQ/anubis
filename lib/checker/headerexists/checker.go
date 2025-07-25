package headerexists

import (
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
)

func New(key string) checker.Interface {
	return headerExistsChecker{
		header: strings.TrimSpace(http.CanonicalHeaderKey(key)),
		hash:   internal.FastHash(key),
	}
}

type headerExistsChecker struct {
	header, hash string
}

func (hec headerExistsChecker) Check(r *http.Request) (bool, error) {
	if r.Header.Get(hec.header) != "" {
		return true, nil
	}

	return false, nil
}

func (hec headerExistsChecker) Hash() string {
	return hec.hash
}
