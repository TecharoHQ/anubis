package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	countryCodeRegexp = regexp.MustCompile(`^[a-zA-Z]{2}$`)

	ErrNotCountryCode = errors.New("config.Bot: invalid country code")
)

type GeoIP struct {
	Countries []string `json:"countries"`
}

func (g *GeoIP) Valid() error {
	var errs []error

	for i, cc := range g.Countries {
		if !countryCodeRegexp.MatchString(cc) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrNotCountryCode, cc))
		}

		g.Countries[i] = strings.ToLower(cc)
	}

	if len(errs) != 0 {
		return fmt.Errorf("bot.GeoIP: invalid GeoIP settings: %w", errors.Join(errs...))
	}

	return nil
}
