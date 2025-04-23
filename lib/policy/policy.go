package policy

import (
	"errors"
	"fmt"
	"io"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/util/yaml"

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
	if err := yaml.NewYAMLToJSONDecoder(fin).Decode(&c); err != nil {
		return nil, fmt.Errorf("can't parse policy config YAML %s: %w", fname, err)
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
			Name:   b.Name,
			Action: b.Action,
		}

		cl := CheckerList{}

		if len(b.RemoteAddr) > 0 {
			c, err := NewRemoteAddrChecker(b.RemoteAddr)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while processing rule %s remote addr set: %w", b.Name, err))
			} else {
				cl = append(cl, c)
			}
		}

		if b.UserAgentRegex != nil {
			c, err := NewUserAgentChecker(*b.UserAgentRegex)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while processing rule %s user agent regex: %w", b.Name, err))
			} else {
				cl = append(cl, c)
			}
		}

		if b.PathRegex != nil {
			c, err := NewPathChecker(*b.PathRegex)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while processing rule %s path regex: %w", b.Name, err))
			} else {
				cl = append(cl, c)
			}
		}

		if len(b.HeadersRegex) > 0 {
			c, err := NewHeadersChecker(b.HeadersRegex)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while processing rule %s headers regex map: %w", b.Name, err))
			} else {
				cl = append(cl, c)
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

		parsedBot.Rules = cl

		result.Bots = append(result.Bots, parsedBot)
	}

	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("errors validating policy config JSON %s: %w", fname, errors.Join(validationErrs...))
	}

	result.DNSBL = c.DNSBL

	return result, nil
}
