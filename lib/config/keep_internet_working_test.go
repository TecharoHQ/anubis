package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type keepInternetWorkingRule struct {
	Name      string `yaml:"name"`
	PathRegex string `yaml:"path_regex"`
	Action    string `yaml:"action"`
}

func TestKeepInternetWorkingRoutes(t *testing.T) {
	policyPath := filepath.Join("..", "..", "data", "common", "keep-internet-working.yaml")
	contents, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}

	var rules []keepInternetWorkingRule
	if err := yaml.Unmarshal(contents, &rules); err != nil {
		t.Fatalf("unmarshal common routes: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected common route rules")
	}

	type compiledRule struct {
		name string
		re   *regexp.Regexp
	}
	compiled := make([]compiledRule, 0, len(rules))
	for _, rule := range rules {
		if rule.Action != "ALLOW" {
			t.Errorf("rule %q action = %q, want ALLOW", rule.Name, rule.Action)
		}
		re, err := regexp.Compile(rule.PathRegex)
		if err != nil {
			t.Fatalf("compile %q path regex: %v", rule.Name, err)
		}
		compiled = append(compiled, compiledRule{name: rule.Name, re: re})
	}

	tests := []struct {
		path     string
		wantRule string
	}{
		{path: "/.well-known/assetlinks.json", wantRule: "well-known"},
		{path: "/site.webmanifest", wantRule: "web-app-manifest"},
		{path: "/manifest.json", wantRule: "web-app-manifest"},
		{path: "/robots.txt", wantRule: "robots-txt"},
		{path: "/sitemap.xml", wantRule: "sitemap"},

		// Reject malformed variants and lookalike paths; `/.well-known/` remains intentionally prefix-scoped.
		{path: "/favicon"},
		{path: "/assets/favicon.ico"},
		{path: "/favicon-abc.png"},
		{path: "/favicon-16X16.png"},
		{path: "/favicon-16x16@2x.png"},
		{path: "/favicon-16x16-precomposed.png"},
		{path: "/favicon-1x2x3.png"},
		{path: "/favicon.png.bak"},
		{path: "/favicon.ico/"},
		{path: "/favicon-16x16.png/child"},
		{path: "/favicon-16x16/../admin.png"},
		{path: "//favicon.ico"},
		{path: "/favicon-16x16.PNG"},
		{path: "/apple-touch-icon-extra.png"},
		{path: "/apple-touch-icon-precomposed-extra.png"},
		{path: "/apple-touch-icon-180x180-precomposed-extra.png"},
		{path: "/apple-touch-icon-180x180@2x.png"},
		{path: "/mstile.png"},
		{path: "/mstile-150.png"},
		{path: "/mstile-150x150-extra.png"},
		{path: "/mstile-150x150/../../admin.png"},
		{path: "/foo/site.webmanifest"},
		{path: "/site.webmanifest.bak"},
		{path: "/manifest.xml"},
		{path: "/robots.txt.bak"},
		{path: "/sitemap-1.xml"},
		{path: "/sitemap.xml.bak"},
		{path: "/foo/.well-known/assetlinks.json"},
	}

	// Exercise every allowed extension against each supported icon filename form.
	for _, ext := range []string{"bmp", "gif", "ico", "jpg", "jpeg", "png", "ppm", "tif", "tiff", "svg", "webp", "avif"} {
		for _, icon := range []struct {
			name string
			rule string
		}{
			{name: "favicon", rule: "favicon"},
			{name: "favicon-16x16", rule: "favicon"},
			{name: "favicon-32x32", rule: "favicon"},
			{name: "apple-touch-icon", rule: "favicon"},
			{name: "apple-touch-icon-precomposed", rule: "favicon"},
			{name: "apple-touch-icon-precomposed-180x180", rule: "favicon"},
			{name: "apple-touch-icon-180x180", rule: "favicon"},
			{name: "apple-touch-icon-180x180-precomposed", rule: "favicon"},
			{name: "mstile-150x150", rule: "favicon"},
		} {
			tests = append(tests, struct {
				path     string
				wantRule string
			}{path: "/" + icon.name + "." + ext, wantRule: icon.rule})
		}
	}

	for _, tc := range tests {
		name := strings.ReplaceAll(strings.TrimPrefix(tc.path, "/"), "/", "__")
		t.Run(name, func(t *testing.T) {
			var matches []string
			for _, rule := range compiled {
				if rule.re.MatchString(tc.path) {
					matches = append(matches, rule.name)
				}
			}

			if tc.wantRule == "" {
				if len(matches) != 0 {
					t.Fatalf("path %q unexpectedly matched allow rules %v", tc.path, matches)
				}
				return
			}
			if len(matches) != 1 || matches[0] != tc.wantRule {
				t.Fatalf("path %q matched %v, want only %q", tc.path, matches, tc.wantRule)
			}
		})
	}
}
