package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yl2chen/cidranger"

	"github.com/TecharoHQ/anubis/lib/policy/config"
)

var (
	PolicyApplications = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_policy_results",
		Help: "The results of each policy rule",
	}, []string{"rule", "action"})
)

type ParsedConfig struct {
	orig config.Config

	Bots              []Bot
	DNSBL             bool
	DefaultDifficulty int
}

func NewParsedConfig(orig config.Config) *ParsedConfig {
	return &ParsedConfig{
		orig: orig,
	}
}

func ParseConfig(fin io.Reader, fname string, defaultDifficulty int) (*ParsedConfig, error) {
	var c config.Config
	if err := json.NewDecoder(fin).Decode(&c); err != nil {
		return nil, fmt.Errorf("can't parse policy config JSON %s: %w", fname, err)
	}

	if err := c.Valid(); err != nil {
		return nil, err
	}

	var validationErrs []error

	result := NewParsedConfig(c)
	result.DefaultDifficulty = defaultDifficulty

	for _, b := range c.Bots {
		if berr := b.Valid(); berr != nil {
			validationErrs = append(validationErrs, berr)
			continue
		}

		parsedBot := Bot{
			Name:    b.Name,
			Action:  b.Action,
			Headers: map[string]*regexp.Regexp{},
		}

		if len(b.RemoteAddr) > 0 {
			parsedBot.Ranger = cidranger.NewPCTrieRanger()

			for _, cidr := range b.RemoteAddr {
				_, rng, err := net.ParseCIDR(cidr)
				if err != nil {
					return nil, fmt.Errorf("[unexpected] range %s not parsing: %w", cidr, err)
				}

				parsedBot.Ranger.Insert(cidranger.NewBasicRangerEntry(*rng))
			}
		}

		if b.UserAgentRegex != nil {
			userAgent, err := regexp.Compile(*b.UserAgentRegex)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while compiling user agent regexp: %w", err))
				continue
			} else {
				parsedBot.UserAgent = userAgent
			}
		}

		if b.PathRegex != nil {
			path, err := regexp.Compile(*b.PathRegex)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while compiling path regexp: %w", err))
				continue
			} else {
				parsedBot.Path = path
			}
		}

		if len(b.HeadersRegex) > 0 {
			for name, expr := range b.HeadersRegex {
				if name == "" {
					continue
				}

				header, err := regexp.Compile(expr)
				if err != nil {
					validationErrs = append(validationErrs, fmt.Errorf("while compiling header regexp: %w", err))
					continue
				} else {
					parsedBot.Headers[name] = header
				}
			}
		}

		if b.Challenge == nil {
			parsedBot.Challenge = &config.ChallengeRules{
				Difficulty: defaultDifficulty,
				ReportAs:   defaultDifficulty,
				Algorithm:  config.AlgorithmFast,
			}
		} else {
			parsedBot.Challenge = b.Challenge
			if parsedBot.Challenge.Algorithm == config.AlgorithmUnknown {
				parsedBot.Challenge.Algorithm = config.AlgorithmFast
			}
		}

		result.Bots = append(result.Bots, parsedBot)
	}

	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("errors validating policy config JSON %s: %w", fname, errors.Join(validationErrs...))
	}

	result.DNSBL = c.DNSBL

	return result, nil
}
