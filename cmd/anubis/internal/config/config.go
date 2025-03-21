package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
)

type Rule string

const (
	RuleUnknown   = ""
	RuleAllow     = "ALLOW"
	RuleDeny      = "DENY"
	RuleChallenge = "CHALLENGE"
)

type Bot struct {
	Name           string   `json:"name"`
	UserAgentRegex *string  `json:"user_agent_regex"`
	PathRegex      *string  `json:"path_regex"`
	RemoteAddr     []string `json:"remote_addresses"`
	Action         Rule     `json:"action"`
}

var (
	ErrNoBotRulesDefined                      = errors.New("config: must define at least one (1) bot rule")
	ErrBotMustHaveName                        = errors.New("config.Bot: must set name")
	ErrBotMustHaveUserAgentOrPathOrRemoteAddr = errors.New("config.Bot: must set either user_agent_regex, path_regex, remote_addresses")
	ErrBotMustHaveUserAgentOrPathNotBoth      = errors.New("config.Bot: must set either user_agent_regex, path_regex, and not both")
	ErrUnknownAction                          = errors.New("config.Bot: unknown action")
	ErrInvalidUserAgentRegex                  = errors.New("config.Bot: invalid user agent regex")
	ErrInvalidPathRegex                       = errors.New("config.Bot: invalid path regex")
	ErrInvalidCIDR                            = errors.New("config.Bot: invalid CIDR")
)

func (b Bot) Valid() error {
	var errs []error

	if b.Name == "" {
		errs = append(errs, ErrBotMustHaveName)
	}

	if b.UserAgentRegex == nil && b.PathRegex == nil && (b.RemoteAddr == nil || len(b.RemoteAddr) == 0) {
		errs = append(errs, ErrBotMustHaveUserAgentOrPathOrRemoteAddr)
	}

	if b.UserAgentRegex != nil && b.PathRegex != nil {
		errs = append(errs, ErrBotMustHaveUserAgentOrPathNotBoth)
	}

	if b.UserAgentRegex != nil {
		if _, err := regexp.Compile(*b.UserAgentRegex); err != nil {
			errs = append(errs, ErrInvalidUserAgentRegex, err)
		}
	}

	if b.RemoteAddr != nil && len(b.RemoteAddr) > 0 {
		for _, cidr := range b.RemoteAddr {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				errs = append(errs, ErrInvalidCIDR, err)
			}
		}
	}

	if b.PathRegex != nil {
		if _, err := regexp.Compile(*b.PathRegex); err != nil {
			errs = append(errs, ErrInvalidPathRegex, err)
		}
	}

	switch b.Action {
	case RuleAllow, RuleChallenge, RuleDeny:
		// okay
	default:
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownAction, b.Action))
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: bot entry for %q is not valid:\n%w", b.Name, errors.Join(errs...))
	}

	return nil
}

type Config struct {
	Bots  []Bot `json:"bots"`
	DNSBL bool  `json:"dnsbl"`
}

func (c Config) Valid() error {
	var errs []error

	if len(c.Bots) == 0 {
		errs = append(errs, ErrNoBotRulesDefined)
	}

	for _, b := range c.Bots {
		if err := b.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config is not valid:\n%w", errors.Join(errs...))
	}

	return nil
}
