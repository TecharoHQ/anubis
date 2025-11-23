package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/data"
	"k8s.io/apimachinery/pkg/util/yaml"
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
	ErrRegexEndsWithNewline              = errors.New("config.Bot: regular expression ends with newline (try >- instead of > in yaml)")
	ErrInvalidImportStatement            = errors.New("config.ImportStatement: invalid source file")
	ErrCantSetBotAndImportValuesAtOnce   = errors.New("config.BotOrImport: can't set bot rules and import values at the same time")
	ErrMustSetBotOrImportRules           = errors.New("config.BotOrImport: rule definition is invalid, you must set either bot rules or an import statement, not both")
	ErrStatusCodeNotValid                = errors.New("config.StatusCode: status code not valid, must be between 100 and 599")
)

type Rule string

const (
	RuleUnknown   Rule = ""
	RuleAllow     Rule = "ALLOW"
	RuleDeny      Rule = "DENY"
	RuleChallenge Rule = "CHALLENGE"
	RuleWeigh     Rule = "WEIGH"
	RuleBenchmark Rule = "DEBUG_BENCHMARK"
)

func (r Rule) Valid() error {
	switch r {
	case RuleAllow, RuleDeny, RuleChallenge, RuleWeigh, RuleBenchmark:
		return nil
	default:
		return ErrUnknownAction
	}
}

const DefaultAlgorithm = "fast"

type BotConfig struct {
	UserAgentRegex *string           `json:"user_agent_regex,omitempty" yaml:"user_agent_regex,omitempty"`
	PathRegex      *string           `json:"path_regex,omitempty" yaml:"path_regex,omitempty"`
	HeadersRegex   map[string]string `json:"headers_regex,omitempty" yaml:"headers_regex,omitempty"`
	Expression     *ExpressionOrList `json:"expression,omitempty" yaml:"expression,omitempty"`
	Challenge      *ChallengeRules   `json:"challenge,omitempty" yaml:"challenge,omitempty"`
	Weight         *Weight           `json:"weight,omitempty" yaml:"weight,omitempty"`

	// Thoth features
	GeoIP *GeoIP `json:"geoip,omitempty"`
	ASNs  *ASNs  `json:"asns,omitempty"`

	Name       string   `json:"name" yaml:"name"`
	Action     Rule     `json:"action" yaml:"action"`
	RemoteAddr []string `json:"remote_addresses,omitempty" yaml:"remote_addresses,omitempty"`
}

func (b BotConfig) Zero() bool {
	for _, cond := range []bool{
		b.Name != "",
		b.UserAgentRegex != nil,
		b.PathRegex != nil,
		len(b.HeadersRegex) != 0,
		b.Action != "",
		len(b.RemoteAddr) != 0,
		b.Challenge != nil,
		b.GeoIP != nil,
		b.ASNs != nil,
	} {
		if cond {
			return false
		}
	}

	return true
}

func (b *BotConfig) Valid() error {
	var errs []error

	if b.Name == "" {
		errs = append(errs, ErrBotMustHaveName)
	}

	allFieldsEmpty := b.UserAgentRegex == nil &&
		b.PathRegex == nil &&
		len(b.RemoteAddr) == 0 &&
		len(b.HeadersRegex) == 0 &&
		b.ASNs == nil &&
		b.GeoIP == nil

	if allFieldsEmpty && b.Expression == nil {
		errs = append(errs, ErrBotMustHaveUserAgentOrPath)
	}

	if b.UserAgentRegex != nil && b.PathRegex != nil {
		errs = append(errs, ErrBotMustHaveUserAgentOrPathNotBoth)
	}

	if b.UserAgentRegex != nil {
		if strings.HasSuffix(*b.UserAgentRegex, "\n") {
			errs = append(errs, fmt.Errorf("%w: user agent regex: %q", ErrRegexEndsWithNewline, *b.UserAgentRegex))
		}

		if _, err := regexp.Compile(*b.UserAgentRegex); err != nil {
			errs = append(errs, ErrInvalidUserAgentRegex, err)
		}
	}

	if b.PathRegex != nil {
		if strings.HasSuffix(*b.PathRegex, "\n") {
			errs = append(errs, fmt.Errorf("%w: path regex: %q", ErrRegexEndsWithNewline, *b.PathRegex))
		}

		if _, err := regexp.Compile(*b.PathRegex); err != nil {
			errs = append(errs, ErrInvalidPathRegex, err)
		}
	}

	if len(b.HeadersRegex) > 0 {
		for name, expr := range b.HeadersRegex {
			if name == "" {
				continue
			}

			if strings.HasSuffix(expr, "\n") {
				errs = append(errs, fmt.Errorf("%w: header %s regex: %q", ErrRegexEndsWithNewline, name, expr))
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

	if b.Expression != nil {
		if err := b.Expression.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	switch b.Action {
	case RuleAllow, RuleBenchmark, RuleChallenge, RuleDeny, RuleWeigh:
		// okay
	default:
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownAction, b.Action))
	}

	if b.Action == RuleChallenge && b.Challenge != nil {
		if err := b.Challenge.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if b.Action == RuleWeigh && b.Weight == nil {
		b.Weight = &Weight{Adjust: 5}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: bot entry for %q is not valid:\n%w", b.Name, errors.Join(errs...))
	}

	return nil
}

type ChallengeRules struct {
	Algorithm  string `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	Difficulty int    `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	ReportAs   int    `json:"report_as,omitempty" yaml:"report_as,omitempty"`
}

var (
	ErrChallengeDifficultyTooLow  = errors.New("config.ChallengeRules: difficulty is too low (must be >= 0)")
	ErrChallengeDifficultyTooHigh = errors.New("config.ChallengeRules: difficulty is too high (must be <= 64)")
	ErrChallengeMustHaveAlgorithm = errors.New("config.ChallengeRules: must have algorithm name set")
)

func (cr ChallengeRules) Valid() error {
	var errs []error

	if cr.Algorithm == "" {
		errs = append(errs, ErrChallengeMustHaveAlgorithm)
	}

	if cr.Difficulty < 0 {
		errs = append(errs, fmt.Errorf("%w, got: %d", ErrChallengeDifficultyTooLow, cr.Difficulty))
	}

	if cr.Difficulty > 64 {
		errs = append(errs, fmt.Errorf("%w, got: %d", ErrChallengeDifficultyTooHigh, cr.Difficulty))
	}

	if len(errs) != 0 {
		return fmt.Errorf("config: challenge rules entry is not valid:\n%w", errors.Join(errs...))
	}

	return nil
}

type ImportStatement struct {
	Import string `json:"import"`
	Bots   []BotConfig
}

func (is *ImportStatement) open() (fs.File, error) {
	if strings.HasPrefix(is.Import, "(data)/") {
		fname := strings.TrimPrefix(is.Import, "(data)/")
		fin, err := data.BotPolicies.Open(fname)
		return fin, err
	}

	return os.Open(is.Import)
}

func (is *ImportStatement) load() error {
	fin, err := is.open()
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvalidImportStatement, is.Import, err)
	}
	defer fin.Close()

	var imported []BotOrImport
	var result []BotConfig

	if err := yaml.NewYAMLToJSONDecoder(fin).Decode(&imported); err != nil {
		return fmt.Errorf("can't parse %s: %w", is.Import, err)
	}

	var errs []error

	for _, b := range imported {
		if err := b.Valid(); err != nil {
			errs = append(errs, err)
		}

		if b.ImportStatement != nil {
			result = append(result, b.ImportStatement.Bots...)
		}

		if b.BotConfig != nil {
			result = append(result, *b.BotConfig)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config %s is not valid:\n%w", is.Import, errors.Join(errs...))
	}

	is.Bots = result

	return nil
}

func (is *ImportStatement) Valid() error {
	return is.load()
}

type BotOrImport struct {
	*BotConfig       `json:",inline"`
	*ImportStatement `json:",inline"`
}

func (boi *BotOrImport) Valid() error {
	if boi.BotConfig != nil && boi.ImportStatement != nil {
		return ErrCantSetBotAndImportValuesAtOnce
	}

	if boi.BotConfig != nil {
		return boi.BotConfig.Valid()
	}

	if boi.ImportStatement != nil {
		return boi.ImportStatement.Valid()
	}

	return ErrMustSetBotOrImportRules
}

type StatusCodes struct {
	Challenge int `json:"CHALLENGE"`
	Deny      int `json:"DENY"`
}

func (sc StatusCodes) Valid() error {
	var errs []error

	if sc.Challenge == 0 || (sc.Challenge < 100 && sc.Challenge >= 599) {
		errs = append(errs, fmt.Errorf("%w: challenge is %d", ErrStatusCodeNotValid, sc.Challenge))
	}

	if sc.Deny == 0 || (sc.Deny < 100 && sc.Deny >= 599) {
		errs = append(errs, fmt.Errorf("%w: deny is %d", ErrStatusCodeNotValid, sc.Deny))
	}

	if len(errs) != 0 {
		return fmt.Errorf("status codes not valid:\n%w", errors.Join(errs...))
	}

	return nil
}

type fileConfig struct {
	OpenGraph   openGraphFileConfig `json:"openGraph,omitempty"`
	Impressum   *Impressum          `json:"impressum,omitempty"`
	Store       *Store              `json:"store"`
	Bots        []BotOrImport       `json:"bots"`
	Thresholds  []Threshold         `json:"thresholds"`
	StatusCodes StatusCodes         `json:"status_codes"`
	DNSBL       bool                `json:"dnsbl"`
	DNSTTL      DnsTTL              `json:"dns_ttl"`
	Logging     *Logging            `json:"logging"`
}

func (c *fileConfig) Valid() error {
	var errs []error

	if len(c.Bots) == 0 {
		errs = append(errs, ErrNoBotRulesDefined)
	}

	for i, b := range c.Bots {
		if err := b.Valid(); err != nil {
			errs = append(errs, fmt.Errorf("bot %d: %w", i, err))
		}
	}

	if c.OpenGraph.Enabled {
		if err := c.OpenGraph.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := c.StatusCodes.Valid(); err != nil {
		errs = append(errs, err)
	}

	for i, t := range c.Thresholds {
		if err := t.Valid(); err != nil {
			errs = append(errs, fmt.Errorf("threshold %d: %w", i, err))
		}
	}

	if err := c.Logging.Valid(); err != nil {
		errs = append(errs, err)
	}

	if c.Store != nil {
		if err := c.Store.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config is not valid:\n%w", errors.Join(errs...))
	}

	return nil
}

func Load(fin io.Reader, fname string) (*Config, error) {
	c := &fileConfig{
		StatusCodes: StatusCodes{
			Challenge: http.StatusOK,
			Deny:      http.StatusOK,
		},
		DNSTTL: DnsTTL{
			Forward: 300,
			Reverse: 300,
		},
		Store: &Store{
			Backend: "memory",
		},
		Logging: (Logging{}).Default(),
	}

	if err := yaml.NewYAMLToJSONDecoder(fin).Decode(&c); err != nil {
		return nil, fmt.Errorf("can't parse policy config YAML %s: %w", fname, err)
	}

	if err := c.Valid(); err != nil {
		return nil, err
	}

	result := &Config{
		DNSBL:  c.DNSBL,
		DNSTTL: c.DNSTTL,
		OpenGraph: OpenGraph{
			Enabled:      c.OpenGraph.Enabled,
			ConsiderHost: c.OpenGraph.ConsiderHost,
			Override:     c.OpenGraph.Override,
		},
		StatusCodes: c.StatusCodes,
		Store:       c.Store,
		Logging:     c.Logging,
	}

	if c.OpenGraph.TimeToLive != "" {
		// XXX(Xe): already validated in Valid()
		ogTTL, _ := time.ParseDuration(c.OpenGraph.TimeToLive)
		result.OpenGraph.TimeToLive = ogTTL
	}

	var validationErrs []error

	for _, boi := range c.Bots {
		if boi.ImportStatement != nil {
			if err := boi.load(); err != nil {
				validationErrs = append(validationErrs, err)
				continue
			}

			result.Bots = append(result.Bots, boi.ImportStatement.Bots...)
		}

		if boi.BotConfig != nil {
			if err := boi.BotConfig.Valid(); err != nil {
				validationErrs = append(validationErrs, err)
				continue
			}

			result.Bots = append(result.Bots, *boi.BotConfig)
		}
	}

	if c.Impressum != nil {
		if err := c.Impressum.Valid(); err != nil {
			validationErrs = append(validationErrs, err)
		}

		result.Impressum = c.Impressum
	}

	if len(c.Thresholds) == 0 {
		c.Thresholds = DefaultThresholds
	}

	for _, t := range c.Thresholds {
		if err := t.Valid(); err != nil {
			validationErrs = append(validationErrs, err)
			continue
		}

		result.Thresholds = append(result.Thresholds, t)
	}

	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("errors validating policy config %s: %w", fname, errors.Join(validationErrs...))
	}

	return result, nil
}

type DnsTTL struct {
	Forward int `json:"forward"`
	Reverse int `json:"reverse"`
}

func (sc DnsTTL) Valid() error {
	var errs []error

	if sc.Forward < 0 {
		errs = append(errs, fmt.Errorf("%w: forward TTL is %d", ErrStatusCodeNotValid, sc.Forward))
	}

	if sc.Reverse < 0 {
		errs = append(errs, fmt.Errorf("%w: reverse TTL is %d", ErrStatusCodeNotValid, sc.Reverse))
	}

	if len(errs) != 0 {
		return fmt.Errorf("dns TTL values not valid:\n%w", errors.Join(errs...))
	}

	return nil
}

type Config struct {
	Impressum   *Impressum
	Store       *Store
	OpenGraph   OpenGraph
	Bots        []BotConfig
	Thresholds  []Threshold
	StatusCodes StatusCodes
	Logging     *Logging
	DNSBL       bool
	DNSTTL      DnsTTL
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
