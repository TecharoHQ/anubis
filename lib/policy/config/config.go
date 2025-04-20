package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
)

var (
	ErrNoBotRulesDefined                 = errors.New("config: must define at least one (1) bot rule")
	ErrBotMustHaveName                   = errors.New("config.Bot: must set name")
	ErrBotMustHaveUserAgentOrPath        = errors.New("config.Bot: must set either user_agent_regex, path_regex, headers_regex, or remote_addresses")
	ErrBotMustHaveUserAgentOrPathNotBoth = errors.New("config.Bot: must set either user_agent_regex, path_regex, and not both")
	ErrUnknownAction                     = errors.New("config.Bot: unknown action")
	ErrInvalidUserAgentRegex             = errors.New("config.Bot: invalid user agent regex")
	ErrInvalidPathRegex                  = errors.New("config.Bot: invalid path regex")
	ErrInvalidHeadersRegex               = errors.New("config.Bot: invalid headers regex")
	ErrInvalidCIDR                       = errors.New("config.Bot: invalid CIDR")
)

type Rule string

const (
	RuleUnknown   Rule = ""
	RuleAllow     Rule = "ALLOW"
	RuleDeny      Rule = "DENY"
	RuleChallenge Rule = "CHALLENGE"
	RuleBenchmark Rule = "DEBUG_BENCHMARK"
)

type Algorithm string

const (
	AlgorithmUnknown Algorithm = ""
	AlgorithmFast    Algorithm = "fast"
	AlgorithmSlow    Algorithm = "slow"
)

type BotConfig struct {
	Name           string            `json:"name"`
	UserAgentRegex *string           `json:"user_agent_regex"`
	PathRegex      *string           `json:"path_regex"`
	HeadersRegex   map[string]string `json:"headers_regex"`
	Action         Rule              `json:"action"`
	RemoteAddr     []string          `json:"remote_addresses"`
	Challenge      *ChallengeRules   `json:"challenge,omitempty"`
}

func (b BotConfig) Valid() error {
	var errs []error

	if b.Name == "" {
		errs = append(errs, ErrBotMustHaveName)
	}

	if b.UserAgentRegex == nil && b.PathRegex == nil && len(b.RemoteAddr) == 0 && len(b.HeadersRegex) == 0 {
		errs = append(errs, ErrBotMustHaveUserAgentOrPath)
	}

	if b.UserAgentRegex != nil && b.PathRegex != nil {
		errs = append(errs, ErrBotMustHaveUserAgentOrPathNotBoth)
	}

	if b.UserAgentRegex != nil {
		if _, err := regexp.Compile(*b.UserAgentRegex); err != nil {
			errs = append(errs, ErrInvalidUserAgentRegex, err)
		}
	}

	if b.PathRegex != nil {
		if _, err := regexp.Compile(*b.PathRegex); err != nil {
			errs = append(errs, ErrInvalidPathRegex, err)
		}
	}

	if len(b.HeadersRegex) > 0 {
		for name, expr := range b.HeadersRegex {
			if name == "" {
				continue
			}

			if _, err := regexp.Compile(expr); err != nil {
				errs = append(errs, ErrInvalidHeadersRegex, err)
			}
		}
	}

	if len(b.RemoteAddr) > 0 {
		for _, cidr := range b.RemoteAddr {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				errs = append(errs, ErrInvalidCIDR, err)
			}
		}
	}

	switch b.Action {
	case RuleAllow, RuleBenchmark, RuleChallenge, RuleDeny:
		// okay
	default:
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownAction, b.Action))
	}

	if b.Action == RuleChallenge && b.Challenge != nil {
		if err := b.Challenge.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: bot entry for %q is not valid:\n%w", b.Name, errors.Join(errs...))
	}

	return nil
}

type ChallengeRules struct {
	Difficulty int       `json:"difficulty"`
	ReportAs   int       `json:"report_as"`
	Algorithm  Algorithm `json:"algorithm"`
}

var (
	ErrChallengeRuleHasWrongAlgorithm = errors.New("config.Bot.ChallengeRules: algorithm is invalid")
	ErrChallengeDifficultyTooLow      = errors.New("config.Bot.ChallengeRules: difficulty is too low (must be >= 1)")
	ErrChallengeDifficultyTooHigh     = errors.New("config.Bot.ChallengeRules: difficulty is too high (must be <= 64)")
)

func (cr ChallengeRules) Valid() error {
	var errs []error

	if cr.Difficulty < 1 {
		errs = append(errs, fmt.Errorf("%w, got: %d", ErrChallengeDifficultyTooLow, cr.Difficulty))
	}

	if cr.Difficulty > 64 {
		errs = append(errs, fmt.Errorf("%w, got: %d", ErrChallengeDifficultyTooHigh, cr.Difficulty))
	}

	switch cr.Algorithm {
	case AlgorithmFast, AlgorithmSlow, AlgorithmUnknown:
		// do nothing, it's all good
	default:
		errs = append(errs, fmt.Errorf("%w: %q", ErrChallengeRuleHasWrongAlgorithm, cr.Algorithm))
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: challenge rules entry is not valid:\n%w", errors.Join(errs...))
	}

	return nil
}

type Config struct {
	Bots  []BotConfig `json:"bots"`
	DNSBL bool        `json:"dnsbl"`
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
