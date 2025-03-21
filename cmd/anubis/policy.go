package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"regexp"

	"github.com/TecharoHQ/anubis/cmd/anubis/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/yl2chen/cidranger"
)

var (
	policyApplications = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_policy_results",
		Help: "The results of each policy rule",
	}, []string{"rule", "action"})
)

type ParsedConfig struct {
	orig config.Config

	Bots  []Bot
	DNSBL bool
}

type Bot struct {
	Name      string
	UserAgent *regexp.Regexp
	Path      *regexp.Regexp
	Action    config.Rule `json:"action"`
	Challenge *config.ChallengeRules
	Ranger    cidranger.Ranger
}

func (b Bot) Hash() (string, error) {
	var pathRex string
	if b.Path != nil {
		pathRex = b.Path.String()
	}
	var userAgentRex string
	if b.UserAgent != nil {
		userAgentRex = b.UserAgent.String()
	}

	return sha256sum(fmt.Sprintf("%s::%s::%s", b.Name, pathRex, userAgentRex))
}

func parseConfig(fin io.Reader, fname string, defaultDifficulty int) (*ParsedConfig, error) {
	var c config.Config
	if err := json.NewDecoder(fin).Decode(&c); err != nil {
		return nil, fmt.Errorf("can't parse policy config JSON %s: %w", fname, err)
	}

	if err := c.Valid(); err != nil {
		return nil, err
	}

	var err error

	result := &ParsedConfig{
		orig: c,
	}

	for _, b := range c.Bots {
		if berr := b.Valid(); berr != nil {
			err = errors.Join(err, berr)
			continue
		}

		var botParseErr error
		parsedBot := Bot{
			Name:   b.Name,
			Action: b.Action,
		}

		if b.RemoteAddr != nil && len(b.RemoteAddr) > 0 {
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
				botParseErr = errors.Join(botParseErr, fmt.Errorf("while compiling user agent regexp: %w", err))
				continue
			} else {
				parsedBot.UserAgent = userAgent
			}
		}

		if b.PathRegex != nil {
			path, err := regexp.Compile(*b.PathRegex)
			if err != nil {
				botParseErr = errors.Join(botParseErr, fmt.Errorf("while compiling path regexp: %w", err))
				continue
			} else {
				parsedBot.Path = path
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

	if err != nil {
		return nil, fmt.Errorf("errors validating policy config JSON %s: %w", fname, err)
	}

	result.DNSBL = c.DNSBL

	return result, nil
}

type CheckResult struct {
	Name string
	Rule config.Rule
}

func (cr CheckResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", cr.Name),
		slog.String("rule", string(cr.Rule)))
}

func cr(name string, rule config.Rule) CheckResult {
	return CheckResult{
		Name: name,
		Rule: rule,
	}
}

func (s *Server) checkRemoteAddress(b Bot, addr net.IP) bool {
	if b.Ranger == nil {
		return false
	}

	ok, err := b.Ranger.Contains(addr)
	if err != nil {
		log.Panicf("[unexpected] something very funky is going on, %q does not have a calculable network number: %v", addr.String(), err)
	}

	return ok
}

// Check evaluates the list of rules, and returns the result
func (s *Server) check(r *http.Request) (CheckResult, *Bot, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return zilch[CheckResult](), nil, fmt.Errorf("[misconfiguration] X-Real-Ip header is not set")
	}

	addr := net.ParseIP(host)
	if addr == nil {
		return zilch[CheckResult](), nil, fmt.Errorf("[misconfiguration] %q is not an IP address", host)
	}

	for _, b := range s.policy.Bots {
		if b.UserAgent != nil {
			if uaMatch := b.UserAgent.MatchString(r.UserAgent()); uaMatch || (uaMatch && s.checkRemoteAddress(b, addr)) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
		}

		if b.Path != nil {
			if pathMatch := b.Path.MatchString(r.URL.Path); pathMatch || (pathMatch && s.checkRemoteAddress(b, addr)) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
		}

		if b.Ranger != nil {
			if s.checkRemoteAddress(b, addr) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
		}
	}

	return cr("default/allow", config.RuleAllow), &Bot{
		Challenge: &config.ChallengeRules{
			Difficulty: defaultDifficulty,
			ReportAs:   defaultDifficulty,
			Algorithm:  config.AlgorithmFast,
		},
	}, nil
}
