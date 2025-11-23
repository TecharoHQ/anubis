package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/TecharoHQ/anubis/lib/thoth"
	"github.com/fahedouch/go-logrotate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	_ "github.com/TecharoHQ/anubis/lib/store/all"
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
	Store             store.Interface
	orig              *config.Config
	Impressum         *config.Impressum
	OpenGraph         config.OpenGraph
	Bots              []Bot
	Thresholds        []*Threshold
	StatusCodes       config.StatusCodes
	DefaultDifficulty int
	DNSBL             bool
	Dns               *internal.Dns
	Logger            *slog.Logger
}

func newParsedConfig(orig *config.Config) *ParsedConfig {
	return &ParsedConfig{
		orig:        orig,
		OpenGraph:   orig.OpenGraph,
		StatusCodes: orig.StatusCodes,
		Dns:         internal.NewDNS(orig.DNSTTL.Forward, orig.DNSTTL.Reverse),
	}
}

func ParseConfig(ctx context.Context, fin io.Reader, fname string, defaultDifficulty int, logLevel string) (*ParsedConfig, error) {
	c, err := config.Load(fin, fname)
	if err != nil {
		return nil, err
	}

	var validationErrs []error

	tc, hasThothClient := thoth.FromContext(ctx)

	result := newParsedConfig(c)
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
			c, err := NewCELChecker(b.Expression, result.Dns)
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

			if parsedBot.Challenge.Algorithm == "slow" {
				slog.Warn("use of deprecated algorithm \"slow\" detected, please update this to \"fast\" when possible", "name", parsedBot.Name)
			}
		}

		if b.Weight != nil {
			parsedBot.Weight = b.Weight
		}

		result.Impressum = c.Impressum

		parsedBot.Rules = cl

		result.Bots = append(result.Bots, parsedBot)
	}

	for _, t := range c.Thresholds {
		if t.Challenge != nil && t.Challenge.Algorithm == "slow" {
			slog.Warn("use of deprecated algorithm \"slow\" detected, please update this to \"fast\" when possible", "name", t.Name)
		}

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

	stFac, ok := store.Get(c.Store.Backend)
	switch ok {
	case true:
		store, err := stFac.Build(ctx, c.Store.Parameters)
		if err != nil {
			validationErrs = append(validationErrs, err)
		} else {
			result.Store = store
		}
	case false:
		validationErrs = append(validationErrs, config.ErrUnknownStoreBackend)
	}

	if c.Logging.Level != nil {
		logLevel = c.Logging.Level.String()
	}

	switch c.Logging.Sink {
	case config.LogSinkStdio:
		result.Logger = internal.InitSlog(logLevel, os.Stderr)
	case config.LogSinkFile:
		out := &logrotate.Logger{
			Filename:           c.Logging.Parameters.Filename,
			FilenameTimeFormat: time.RFC3339,
			MaxBytes:           c.Logging.Parameters.MaxBytes,
			MaxAge:             c.Logging.Parameters.MaxAge,
			MaxBackups:         c.Logging.Parameters.MaxBackups,
			LocalTime:          c.Logging.Parameters.UseLocalTime,
			Compress:           c.Logging.Parameters.Compress,
		}

		result.Logger = internal.InitSlog(logLevel, out)
	}

	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("errors validating policy config JSON %s: %w", fname, errors.Join(validationErrs...))
	}

	result.DNSBL = c.DNSBL

	return result, nil
}
