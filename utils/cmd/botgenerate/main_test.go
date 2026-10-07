package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParsePrefixes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input string
		want  []string
		err   error
	}{
		{
			name:  "ipv4 and ipv6",
			input: `{"prefixes":[{"ipv4Prefix":"192.0.2.0/24"},{"ipv6Prefix":"2001:db8::/32"}]}`,
			want:  []string{"192.0.2.0/24", "2001:db8::/32"},
		},
		{
			name:  "ignores unrelated fields",
			input: `{"creationTime":"2026-08-31T14:46:46.000000","notes":"hi","prefixes":[{"ipv4Prefix":"192.0.2.0/24"}]}`,
			want:  []string{"192.0.2.0/24"},
		},
		{
			name:  "no prefixes",
			input: `{"prefixes":[]}`,
			err:   ErrNoPrefixes,
		},
		{
			name:  "empty prefix entry",
			input: `{"prefixes":[{"ipv4Prefix":"192.0.2.0/24"},{}]}`,
		},
		{
			name:  "bare address is not a prefix",
			input: `{"prefixes":[{"ipv4Prefix":"192.0.2.1"}]}`,
		},
		{
			name:  "garbage prefix",
			input: `{"prefixes":[{"ipv4Prefix":"not-an-ip/24"}]}`,
		},
		{
			name:  "not json",
			input: `<html>go away</html>`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePrefixes(strings.NewReader(tt.input))

			if tt.want == nil {
				if err == nil {
					t.Logf("got: %v", got)
					t.Fatal("wanted an error but parsing succeeded")
				}

				if tt.err != nil && !errors.Is(err, tt.err) {
					t.Logf("want: %v", tt.err)
					t.Logf("got:  %v", err)
					t.Error("got wrong error")
				}

				return
			}

			if err != nil {
				t.Fatalf("can't parse prefix list: %v", err)
			}

			if !slices.Equal(got, tt.want) {
				t.Logf("want: %v", tt.want)
				t.Logf("got:  %v", got)
				t.Error("got wrong prefixes")
			}
		})
	}
}

func TestSetRemoteAddresses(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		input   string
		rule    string
		want    []string
		wantOld []string
		err     error
	}{
		{
			name: "block style rule",
			input: `- name: googlebot
  user_agent_regex: \+http\://www\.google\.com/bot\.html
  action: ALLOW
  remote_addresses:
    - 192.0.2.0/24
`,
			rule:    "googlebot",
			want:    []string{"198.51.100.0/24"},
			wantOld: []string{"192.0.2.0/24"},
		},
		{
			name: "picks the right rule in a multi rule file",
			input: `- name: first
  action: ALLOW
  remote_addresses: ["192.0.2.0/24"]
- name: second
  action: ALLOW
  remote_addresses: ["203.0.113.0/24"]
`,
			rule:    "second",
			want:    []string{"198.51.100.0/24"},
			wantOld: []string{"203.0.113.0/24"},
		},
		{
			name: "rule is missing",
			input: `- name: googlebot
  action: ALLOW
  remote_addresses: ["192.0.2.0/24"]
`,
			rule: "bingbot",
			err:  ErrRuleNotFound,
		},
		{
			name: "rule has no remote_addresses",
			input: `- name: yandexbot
  action: ALLOW
`,
			rule: "yandexbot",
			err:  ErrNoRemoteAddresses,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var root yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &root); err != nil {
				t.Fatalf("can't parse test fixture: %v", err)
			}

			old, err := setRemoteAddresses(&root, tt.rule, tt.want)

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Logf("want: %v", tt.err)
					t.Logf("got:  %v", err)
					t.Error("got wrong error")
				}

				return
			}

			if err != nil {
				t.Fatalf("can't set remote addresses: %v", err)
			}

			if !slices.Equal(old, tt.wantOld) {
				t.Logf("want: %v", tt.wantOld)
				t.Logf("got:  %v", old)
				t.Error("got wrong previous addresses")
			}

			data, err := marshal(&root)
			if err != nil {
				t.Fatalf("can't serialize result: %v", err)
			}

			var rules []struct {
				Name       string   `yaml:"name"`
				RemoteAddr []string `yaml:"remote_addresses"`
			}
			if err := yaml.Unmarshal(data, &rules); err != nil {
				t.Fatalf("can't reparse result: %v", err)
			}

			idx := slices.IndexFunc(rules, func(r struct {
				Name       string   `yaml:"name"`
				RemoteAddr []string `yaml:"remote_addresses"`
			}) bool {
				return r.Name == tt.rule
			})
			if idx == -1 {
				t.Fatalf("rule %s vanished from the result:\n%s", tt.rule, data)
			}

			if !slices.Equal(rules[idx].RemoteAddr, tt.want) {
				t.Logf("want: %v", tt.want)
				t.Logf("got:  %v", rules[idx].RemoteAddr)
				t.Error("got wrong addresses in output")
			}
		})
	}
}

func TestSetRemoteAddressesKeepsComments(t *testing.T) {
	t.Parallel()

	const input = `# Indexing for search and Siri
# https://support.apple.com/en-us/119829
- name: applebot
  user_agent_regex: Applebot
  action: ALLOW
  # https://search.developer.apple.com/applebot.json
  remote_addresses: ["17.241.208.160/27"]
  # trailing note
`

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(input), &root); err != nil {
		t.Fatalf("can't parse test fixture: %v", err)
	}

	if _, err := setRemoteAddresses(&root, "applebot", []string{"192.0.2.0/24"}); err != nil {
		t.Fatalf("can't set remote addresses: %v", err)
	}

	data, err := marshal(&root)
	if err != nil {
		t.Fatalf("can't serialize result: %v", err)
	}

	for _, want := range []string{
		"# Indexing for search and Siri",
		"# https://support.apple.com/en-us/119829",
		"# https://search.developer.apple.com/applebot.json",
		"# trailing note",
		"user_agent_regex: Applebot",
	} {
		if !strings.Contains(string(data), want) {
			t.Logf("want to find: %q", want)
			t.Logf("got:\n%s", data)
			t.Error("output lost part of the original file")
		}
	}
}
func TestSetRemoteAddressesKeepsStyle(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "flow style quotes every prefix",
			input: "- name: applebot\n  remote_addresses: [\"192.0.2.0/24\"]\n",
			want:  `remote_addresses: ["198.51.100.0/24", "2001:db8::/32"]`,
		},
		{
			name:  "block style leaves prefixes bare",
			input: "- name: applebot\n  remote_addresses:\n    - 192.0.2.0/24\n",
			want:  "remote_addresses:\n    - 198.51.100.0/24\n    - 2001:db8::/32",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var root yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &root); err != nil {
				t.Fatalf("can't parse test fixture: %v", err)
			}

			if _, err := setRemoteAddresses(&root, "applebot", []string{"198.51.100.0/24", "2001:db8::/32"}); err != nil {
				t.Fatalf("can't set remote addresses: %v", err)
			}

			data, err := marshal(&root)
			if err != nil {
				t.Fatalf("can't serialize result: %v", err)
			}

			if !strings.Contains(string(data), tt.want) {
				t.Logf("want to find:\n%s", tt.want)
				t.Logf("got:\n%s", data)
				t.Error("wrote the prefixes in the wrong style")
			}
		})
	}
}

func TestSelectSources(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input []string
		want  []string
		err   error
	}{
		{
			name:  "no arguments selects everything",
			input: nil,
			want:  allSourceNames(),
		},
		{
			name:  "by name",
			input: []string{"googlebot", "bingbot"},
			want:  []string{"googlebot", "bingbot"},
		},
		{
			name:  "unknown crawler",
			input: []string{"googlebot", "clippy"},
			err:   ErrNoSuchCrawler,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := selectSources(tt.input)

			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Logf("want: %v", tt.err)
					t.Logf("got:  %v", err)
					t.Error("got wrong error")
				}

				return
			}

			if err != nil {
				t.Fatalf("can't select sources: %v", err)
			}

			var names []string
			for _, src := range got {
				names = append(names, src.Name())
			}

			if !slices.Equal(names, tt.want) {
				t.Logf("want: %v", tt.want)
				t.Logf("got:  %v", names)
				t.Error("selected the wrong crawlers")
			}
		})
	}
}

func TestSourcesMatchDataFiles(t *testing.T) {
	t.Parallel()

	const dir = "../../../data/crawlers"

	for _, src := range sources {
		t.Run(src.Name(), func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(dir, src.File))
			if err != nil {
				t.Fatalf("can't read policy file: %v", err)
			}

			var rules []struct {
				Name       string   `yaml:"name"`
				RemoteAddr []string `yaml:"remote_addresses"`
			}
			if err := yaml.Unmarshal(data, &rules); err != nil {
				t.Fatalf("can't parse policy file: %v", err)
			}

			for _, rule := range rules {
				if rule.Name != src.Rule {
					continue
				}

				if len(rule.RemoteAddr) == 0 {
					t.Errorf("rule %s has no remote_addresses to update", src.Rule)
				}

				return
			}

			t.Errorf("%s declares no rule named %q", src.File, src.Rule)
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("wanted User-Agent test-agent, got: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		writePrefixes(t, w)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	fname := filepath.Join(dir, "googlebot.yaml")

	const input = `- name: googlebot
  action: ALLOW
  # source of truth
  remote_addresses:
    - 192.0.2.0/24
`

	if err := os.WriteFile(fname, []byte(input), 0o644); err != nil {
		t.Fatalf("can't write test fixture: %v", err)
	}

	src := Source{File: "googlebot.yaml", Rule: "googlebot", URL: srv.URL}

	t.Run("dry run leaves the file alone", func(t *testing.T) {
		u := &updater{cli: srv.Client(), dataDir: dir, dryRun: true, userAgent: "test-agent"}

		if err := u.update(src); err != nil {
			t.Fatalf("can't update: %v", err)
		}

		got, err := os.ReadFile(fname)
		if err != nil {
			t.Fatalf("can't read file back: %v", err)
		}

		if string(got) != input {
			t.Logf("want:\n%s", input)
			t.Logf("got:\n%s", got)
			t.Error("dry run wrote to the file")
		}
	})

	t.Run("writes the new prefixes", func(t *testing.T) {
		u := &updater{cli: srv.Client(), dataDir: dir, userAgent: "test-agent"}

		if err := u.update(src); err != nil {
			t.Fatalf("can't update: %v", err)
		}

		got, err := os.ReadFile(fname)
		if err != nil {
			t.Fatalf("can't read file back: %v", err)
		}

		for _, want := range []string{"198.51.100.0/24", "2001:db8::/32", "# source of truth"} {
			if !strings.Contains(string(got), want) {
				t.Logf("want to find: %q", want)
				t.Logf("got:\n%s", got)
				t.Error("output is missing something")
			}
		}

		if strings.Contains(string(got), "192.0.2.0/24") {
			t.Logf("got:\n%s", got)
			t.Error("stale prefix survived the update")
		}
	})
}

func TestUpdateRejectsBadFeed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{name: "server error", status: http.StatusInternalServerError, body: "nope", wantErr: ErrUnexpectedStatus},
		{name: "empty feed", status: http.StatusOK, body: `{"prefixes":[]}`, wantErr: ErrNoPrefixes},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Errorf("can't write response: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			dir := t.TempDir()
			fname := filepath.Join(dir, "googlebot.yaml")

			const input = "- name: googlebot\n  action: ALLOW\n  remote_addresses:\n    - 192.0.2.0/24\n"
			if err := os.WriteFile(fname, []byte(input), 0o644); err != nil {
				t.Fatalf("can't write test fixture: %v", err)
			}

			u := &updater{cli: srv.Client(), dataDir: dir, userAgent: "test-agent"}

			err := u.update(Source{File: "googlebot.yaml", Rule: "googlebot", URL: srv.URL})
			if !errors.Is(err, tt.wantErr) {
				t.Logf("want: %v", tt.wantErr)
				t.Logf("got:  %v", err)
				t.Error("got wrong error")
			}

			got, err := os.ReadFile(fname)
			if err != nil {
				t.Fatalf("can't read file back: %v", err)
			}

			if string(got) != input {
				t.Logf("got:\n%s", got)
				t.Error("a failed fetch must not touch the policy file")
			}
		})
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		old, cur    []string
		wantAdded   []string
		wantRemoved []string
	}{
		{
			name: "no change",
			old:  []string{"192.0.2.0/24"},
			cur:  []string{"192.0.2.0/24"},
		},
		{
			name:      "reordering is not a change",
			old:       []string{"192.0.2.0/24", "198.51.100.0/24"},
			cur:       []string{"198.51.100.0/24", "192.0.2.0/24"},
			wantAdded: nil,
		},
		{
			name:        "added and removed",
			old:         []string{"192.0.2.0/24", "203.0.113.0/24"},
			cur:         []string{"192.0.2.0/24", "198.51.100.0/24"},
			wantAdded:   []string{"198.51.100.0/24"},
			wantRemoved: []string{"203.0.113.0/24"},
		},
		{
			name:      "first run",
			cur:       []string{"192.0.2.0/24"},
			wantAdded: []string{"192.0.2.0/24"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			added, removed := diff(tt.old, tt.cur)

			if !slices.Equal(added, tt.wantAdded) {
				t.Logf("want: %v", tt.wantAdded)
				t.Logf("got:  %v", added)
				t.Error("got wrong added prefixes")
			}

			if !slices.Equal(removed, tt.wantRemoved) {
				t.Logf("want: %v", tt.wantRemoved)
				t.Logf("got:  %v", removed)
				t.Error("got wrong removed prefixes")
			}
		})
	}
}

func allSourceNames() []string {
	var names []string
	for _, src := range sources {
		names = append(names, src.Name())
	}
	return names
}

func writePrefixes(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	const body = `{"prefixes":[{"ipv4Prefix":"198.51.100.0/24"},{"ipv6Prefix":"2001:db8::/32"}]}`

	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("can't write response: %v", err)
	}
}
