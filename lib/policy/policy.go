package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/TecharoHQ/anubis/internal/thoth"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	Applications = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_policy_results",
		Help: "The results of each policy rule",
	}, []string{"rule", "action"})

	ErrChallengeRuleHasWrongAlgorithm = errors.New("config.Bot.ChallengeRules: algorithm is invalid")
	warnedAboutThresholds             = &atomic.Bool{}
)

type ParsedConfig struct {
	orig *config.Config

	Bots              []Bot
	Thresholds        []*Threshold
	DNSBL             bool
	DefaultDifficulty int
	StatusCodes       config.StatusCodes
}

func NewParsedConfig(orig *config.Config) *ParsedConfig {
	return &ParsedConfig{
		orig:        orig,
		StatusCodes: orig.StatusCodes,
	}
}

func ParseConfig(ctx context.Context, fin io.Reader, fname string, defaultDifficulty int) (*ParsedConfig, error) {
	c, err := config.Load(fin, fname)
	if err != nil {
		return nil, err
	}

	var validationErrs []error

	tc, hasThothClient := thoth.FromContext(ctx)

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

		cl := checker.List{}

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

		if b.Expression != nil {
			c, err := NewCELChecker(b.Expression)
			if err != nil {
				validationErrs = append(validationErrs, fmt.Errorf("while processing rule %s expressions: %w", b.Name, err))
			} else {
				cl = append(cl, c)
			}
		}

		if b.ASNs != nil {
			if !hasThothClient {
				slog.Warn("You have specified a Thoth specific check but you have no Thoth client configured. Please read https://anubis.techaro.lol/docs/admin/thoth for more information", "check", "asn", "settings", b.ASNs)
				continue
			}

			cl = append(cl, tc.ASNCheckerFor(b.ASNs.Match))
		}

		if b.GeoIP != nil {
			if !hasThothClient {
				slog.Warn("You have specified a Thoth specific check but you have no Thoth client configured. Please read https://anubis.techaro.lol/docs/admin/thoth for more information", "check", "geoip", "settings", b.GeoIP)
				continue
			}

			cl = append(cl, tc.GeoIPCheckerFor(b.GeoIP.Countries))
		}

		if b.Challenge == nil {
			parsedBot.Challenge = &config.ChallengeRules{
				Difficulty: defaultDifficulty,
				ReportAs:   defaultDifficulty,
				Algorithm:  "fast",
			}
		} else {
			parsedBot.Challenge = b.Challenge
			if parsedBot.Challenge.Algorithm == "" {
				parsedBot.Challenge.Algorithm = config.DefaultAlgorithm
			}
		}

		if b.Weight != nil {
			parsedBot.Weight = b.Weight
		}

		parsedBot.Rules = cl

		result.Bots = append(result.Bots, parsedBot)
	}

	for _, t := range c.Thresholds {
		if t.Name == "legacy-anubis-behaviour" && t.Expression.String() == "true" {
			if !warnedAboutThresholds.Load() {
				slog.Warn("configuration file does not contain thresholds, see docs for details on how to upgrade", "fname", fname, "docs_url", "https://anubis.techaro.lol/docs/admin/configuration/thresholds/")
				warnedAboutThresholds.Store(true)
			}

			t.Challenge.Difficulty = defaultDifficulty
			t.Challenge.ReportAs = defaultDifficulty
		}

		threshold, err := ParsedThresholdFromConfig(t)
		if err != nil {
			validationErrs = append(validationErrs, fmt.Errorf("can't compile threshold config for %s: %w", t.Name, err))
			continue
		}

		result.Thresholds = append(result.Thresholds, threshold)
	}

	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("errors validating policy config JSON %s: %w", fname, errors.Join(validationErrs...))
	}

	result.DNSBL = c.DNSBL

	return result, nil
}
