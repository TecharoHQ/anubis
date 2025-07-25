package policy

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/checker/headerexists"
	"github.com/TecharoHQ/anubis/lib/checker/headermatches"
)

func NewHeadersChecker(headermap map[string]string) (checker.Interface, error) {
	var result checker.All
	var errs []error

	var keys []string
	for key := range headermap {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		rexStr := headermap[key]
		if rexStr == ".*" {
			result = append(result, headerexists.New(strings.TrimSpace(key)))
			continue
		}

		c, err := headermatches.New(key, rexStr)
		if err != nil {
			errs = append(errs, fmt.Errorf("while parsing header %s regex %s: %w", key, rexStr, err))
			continue
		}

		result = append(result, c)
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return result, nil
}
