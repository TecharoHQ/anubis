package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"

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
	Name       string
	UserAgent  *regexp.Regexp
	Path       *regexp.Regexp
	RemoteAddr []string
	Action     config.Rule `json:"action"`
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

func parseConfig(fin io.Reader, fname string) (*ParsedConfig, error) {
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
			parsedBot.RemoteAddr = b.RemoteAddr
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

// Check evaluates the list of rules, and returns the result
func (s *Server) check(r *http.Request) (CheckResult, *Bot) {
	for _, b := range s.policy.Bots {
		if b.UserAgent != nil {
			if b.UserAgent.MatchString(r.UserAgent()) {
				return cr("bot/"+b.Name, b.Action), &b
			}
		}

		if b.Path != nil {
			if b.Path.MatchString(r.URL.Path) {
				return cr("bot/"+b.Name, b.Action), &b
			}
		}

		if b.RemoteAddr != nil && len(b.RemoteAddr) > 0 {
			for _, cidrString := range b.RemoteAddr {
				ranger := cidranger.NewPCTrieRanger()

				_, network, _ := net.ParseCIDR(cidrString)
				_ = ranger.Insert(cidranger.NewBasicRangerEntry(*network))

				ipToCheck := [3]string{
					strings.Split(r.RemoteAddr, ":")[0],
					r.Header.Get("X-Real-Ip"),
					r.Header.Get("X-Forwarded-For"),
				}

				for _, ip := range ipToCheck {
					ipInCidr, err := ranger.Contains(net.ParseIP(ip))
					if err != nil {
						continue
					}

					if ipInCidr {
						return cr("bot/"+b.Name, b.Action), &b
					}
				}
			}
		}
	}

	return cr("default/allow", config.RuleAllow), nil
}
