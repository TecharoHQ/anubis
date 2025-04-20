package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func p[V any](v V) *V { return &v }

func TestBotValid(t *testing.T) {
	var tests = []struct {
		name string
		bot  BotConfig
		err  error
	}{
		{
			name: "simple user agent",
			bot: BotConfig{
				Name:           "mozilla-ua",
				Action:         RuleChallenge,
				UserAgentRegex: p("Mozilla"),
			},
			err: nil,
		},
		{
			name: "simple path",
			bot: BotConfig{
				Name:      "well-known-path",
				Action:    RuleAllow,
				PathRegex: p("^/.well-known/.*$"),
			},
			err: nil,
		},
		{
			name: "no rule name",
			bot: BotConfig{
				Action:         RuleChallenge,
				UserAgentRegex: p("Mozilla"),
			},
			err: ErrBotMustHaveName,
		},
		{
			name: "no rule matcher",
			bot: BotConfig{
				Name:   "broken-rule",
				Action: RuleAllow,
			},
			err: ErrBotMustHaveUserAgentOrPath,
		},
		{
			name: "both user-agent and path",
			bot: BotConfig{
				Name:           "path-and-user-agent",
				Action:         RuleDeny,
				UserAgentRegex: p("Mozilla"),
				PathRegex:      p("^/.secret-place/.*$"),
			},
			err: ErrBotMustHaveUserAgentOrPathNotBoth,
		},
		{
			name: "unknown action",
			bot: BotConfig{
				Name:           "Unknown action",
				Action:         RuleUnknown,
				UserAgentRegex: p("Mozilla"),
			},
			err: ErrUnknownAction,
		},
		{
			name: "invalid user agent regex",
			bot: BotConfig{
				Name:           "mozilla-ua",
				Action:         RuleChallenge,
				UserAgentRegex: p("a(b"),
			},
			err: ErrInvalidUserAgentRegex,
		},
		{
			name: "invalid path regex",
			bot: BotConfig{
				Name:      "mozilla-ua",
				Action:    RuleChallenge,
				PathRegex: p("a(b"),
			},
			err: ErrInvalidPathRegex,
		},
		{
			name: "invalid headers regex",
			bot: BotConfig{
				Name:   "mozilla-ua",
				Action: RuleChallenge,
				HeadersRegex: map[string]string{
					"Content-Type": "a(b",
				},
				PathRegex: p("a(b"),
			},
			err: ErrInvalidHeadersRegex,
		},
		{
			name: "challenge difficulty too low",
			bot: BotConfig{
				Name:      "mozilla-ua",
				Action:    RuleChallenge,
				PathRegex: p("Mozilla"),
				Challenge: &ChallengeRules{
					Difficulty: 0,
					ReportAs:   4,
					Algorithm:  "fast",
				},
			},
			err: ErrChallengeDifficultyTooLow,
		},
		{
			name: "challenge difficulty too high",
			bot: BotConfig{
				Name:      "mozilla-ua",
				Action:    RuleChallenge,
				PathRegex: p("Mozilla"),
				Challenge: &ChallengeRules{
					Difficulty: 420,
					ReportAs:   4,
					Algorithm:  "fast",
				},
			},
			err: ErrChallengeDifficultyTooHigh,
		},
		{
			name: "challenge wrong algorithm",
			bot: BotConfig{
				Name:      "mozilla-ua",
				Action:    RuleChallenge,
				PathRegex: p("Mozilla"),
				Challenge: &ChallengeRules{
					Difficulty: 420,
					ReportAs:   4,
					Algorithm:  "high quality rips",
				},
			},
			err: ErrChallengeRuleHasWrongAlgorithm,
		},
		{
			name: "invalid cidr range",
			bot: BotConfig{
				Name:       "mozilla-ua",
				Action:     RuleAllow,
				RemoteAddr: []string{"0.0.0.0/33"},
			},
			err: ErrInvalidCIDR,
		},
		{
			name: "only filter by IP range",
			bot: BotConfig{
				Name:       "mozilla-ua",
				Action:     RuleAllow,
				RemoteAddr: []string{"0.0.0.0/0"},
			},
			err: nil,
		},
		{
			name: "filter by user agent and IP range",
			bot: BotConfig{
				Name:           "mozilla-ua",
				Action:         RuleAllow,
				UserAgentRegex: p("Mozilla"),
				RemoteAddr:     []string{"0.0.0.0/0"},
			},
			err: nil,
		},
		{
			name: "filter by path and IP range",
			bot: BotConfig{
				Name:       "mozilla-ua",
				Action:     RuleAllow,
				PathRegex:  p("^.*$"),
				RemoteAddr: []string{"0.0.0.0/0"},
			},
			err: nil,
		},
	}

	for _, cs := range tests {
		cs := cs
		t.Run(cs.name, func(t *testing.T) {
			err := cs.bot.Valid()
			if err == nil && cs.err == nil {
				return
			}

			if err == nil && cs.err != nil {
				t.Errorf("didn't get an error, but wanted: %v", cs.err)
			}

			if !errors.Is(err, cs.err) {
				t.Logf("got wrong error from Valid()")
				t.Logf("wanted: %v", cs.err)
				t.Logf("got:    %v", err)
				t.Errorf("got invalid error from check")
			}
		})
	}
}

func TestConfigValidKnownGood(t *testing.T) {
	finfos, err := os.ReadDir("testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("testdata", "good", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			var c Config
			if err := json.NewDecoder(fin).Decode(&c); err != nil {
				t.Fatalf("can't decode file: %v", err)
			}

			if err := c.Valid(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigValidBad(t *testing.T) {
	finfos, err := os.ReadDir("testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("testdata", "bad", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			var c Config
			if err := json.NewDecoder(fin).Decode(&c); err != nil {
				t.Fatalf("can't decode file: %v", err)
			}

			if err := c.Valid(); err == nil {
				t.Fatal("validation should have failed but didn't somehow")
			} else {
				t.Log(err)
			}
		})
	}
}
