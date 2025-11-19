package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/data"
	. "github.com/TecharoHQ/anubis/lib/config"
)

func p[V any](v V) *V { return &v }

func TestBotValid(t *testing.T) {
	var tests = []struct {
		bot  BotConfig
		err  error
		name string
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
					Difficulty: -1,
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
		{
			name: "weight rule without weight",
			bot: BotConfig{
				Name:           "weight-adjust-if-mozilla",
				Action:         RuleWeigh,
				UserAgentRegex: p("Mozilla"),
			},
		},
		{
			name: "weight rule with weight adjust",
			bot: BotConfig{
				Name:           "weight-adjust-if-mozilla",
				Action:         RuleWeigh,
				UserAgentRegex: p("Mozilla"),
				Weight: &Weight{
					Adjust: 5,
				},
			},
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

			c, err := Load(fin, st.Name())
			if err != nil {
				t.Fatal(err)
			}

			if err := c.Valid(); err != nil {
				t.Error(err)
			}

			if len(c.Bots) == 0 {
				t.Error("wanted more than 0 bots, got zero")
			}
		})
	}
}

func TestImportStatement(t *testing.T) {
	type testCase struct {
		err        error
		name       string
		importPath string
	}

	var tests []testCase

	for _, folderName := range []string{
		"apps",
		"bots",
		"common",
		"crawlers",
		"meta",
	} {
		if err := fs.WalkDir(data.BotPolicies, folderName, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() == "README.md" {
				return nil
			}

			tests = append(tests, testCase{
				name:       "(data)/" + path,
				importPath: "(data)/" + path,
				err:        nil,
			})

			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := &ImportStatement{
				Import: tt.importPath,
			}

			if err := is.Valid(); err != nil {
				t.Errorf("validation error: %v", err)
			}

			if len(is.Bots) == 0 {
				t.Error("wanted bot definitions, but got none")
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

			_, err = Load(fin, filepath.Join("testdata", "bad", st.Name()))
			if err == nil {
				t.Fatal("validation should have failed but didn't somehow")
			} else {
				t.Log(err)
			}
		})
	}
}

func TestBotConfigZero(t *testing.T) {
	var b BotConfig
	if !b.Zero() {
		t.Error("zero value config.BotConfig is not zero value")
	}

	b.Name = "hi"
	if b.Zero() {
		t.Error("config.BotConfig with name is zero value")
	}

	b.UserAgentRegex = p(".*")
	if b.Zero() {
		t.Error("config.BotConfig with user agent regex is zero value")
	}

	b.PathRegex = p(".*")
	if b.Zero() {
		t.Error("config.BotConfig with path regex is zero value")
	}

	b.HeadersRegex = map[string]string{"hi": "there"}
	if b.Zero() {
		t.Error("config.BotConfig with headers regex is zero value")
	}

	b.Action = RuleAllow
	if b.Zero() {
		t.Error("config.BotConfig with action is zero value")
	}

	b.RemoteAddr = []string{"::/0"}
	if b.Zero() {
		t.Error("config.BotConfig with remote addresses is zero value")
	}

	b.Challenge = &ChallengeRules{
		Difficulty: 4,
		ReportAs:   4,
		Algorithm:  DefaultAlgorithm,
	}
	if b.Zero() {
		t.Error("config.BotConfig with challenge rules is zero value")
	}
}
