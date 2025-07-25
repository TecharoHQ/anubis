package headermatches

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/TecharoHQ/anubis/lib/checker"
)

type Checker struct {
	header string
	regexp *regexp.Regexp
	hash   string
}

func (c *Checker) Check(r *http.Request) (bool, error) {
	if c.regexp.MatchString(r.Header.Get(c.header)) {
		return true, nil
	}

	return false, nil
}

func (c *Checker) Hash() string {
	return c.hash
}

func New(key, valueRex string) (checker.Interface, error) {
	fc := fileConfig{
		Header:     key,
		ValueRegex: valueRex,
	}

	if err := fc.Valid(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(fc)
	if err != nil {
		return nil, err
	}

	return Factory{}.Build(context.Background(), json.RawMessage(data))
}
