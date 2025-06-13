package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Test URLs for real robots.txt files
var testRobotsURLs = []struct {
	name string
	url  string
	desc string
}{
	{"Google", "https://www.google.com/robots.txt", "Google's comprehensive robots.txt with many disallows"},
	{"Apple", "https://www.apple.com/robots.txt", "Apple's robots.txt with product-specific rules"},
	{"GitHub", "https://github.com/robots.txt", "GitHub's developer-focused robots.txt"},
	{"Reddit", "https://www.reddit.com/robots.txt", "Reddit's social media robots.txt"},
	{"Wikipedia", "https://en.wikipedia.org/robots.txt", "Wikipedia's educational content robots.txt"},
	{"Twitter", "https://twitter.com/robots.txt", "Twitter's social platform robots.txt"},
	{"Facebook", "https://www.facebook.com/robots.txt", "Facebook's social network robots.txt"},
	{"LinkedIn", "https://www.linkedin.com/robots.txt", "LinkedIn's professional network robots.txt"},
	{"Amazon", "https://www.amazon.com/robots.txt", "Amazon's e-commerce robots.txt"},
	{"Microsoft", "https://www.microsoft.com/robots.txt", "Microsoft's corporate robots.txt"},
}

func TestRealRobotsTxtConversion(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
	}

	for _, test := range testRobotsURLs {
		t.Run(test.name, func(t *testing.T) {
			// Fetch robots.txt
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(test.url)
			if err != nil {
				t.Skipf("Failed to fetch %s: %v", test.url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Skipf("Non-200 status for %s: %d", test.url, resp.StatusCode)
			}

			// Parse robots.txt
			rules, err := parseRobotsTxt(resp.Body)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt from %s: %v", test.url, err)
			}

			// Convert to Anubis rules
			oldPolicyName := *policyName
			*policyName = test.name + "-policy"
			defer func() { *policyName = oldPolicyName }()

			anubisRules := convertToAnubisRules(rules)

			// Validate we got some rules
			if len(anubisRules) == 0 {
				t.Errorf("No rules generated for %s", test.name)
				return
			}

			// Test YAML output
			yamlOutput, err := yaml.Marshal(anubisRules)
			if err != nil {
				t.Fatalf("Failed to marshal YAML for %s: %v", test.name, err)
			}

			if len(yamlOutput) == 0 {
				t.Errorf("Empty YAML output for %s", test.name)
			}

			// Test JSON output
			jsonOutput, err := json.MarshalIndent(anubisRules, "", "  ")
			if err != nil {
				t.Fatalf("Failed to marshal JSON for %s: %v", test.name, err)
			}

			if len(jsonOutput) == 0 {
				t.Errorf("Empty JSON output for %s", test.name)
			}

			// Validate rule structure
			for i, rule := range anubisRules {
				if rule.Name == "" {
					t.Errorf("Rule %d has empty name for %s", i, test.name)
				}

				if rule.Action == "" {
					t.Errorf("Rule %d has empty action for %s", i, test.name)
				}

				validActions := map[string]bool{
					"ALLOW": true, "DENY": true, "CHALLENGE": true, "WEIGH": true,
				}
				if !validActions[rule.Action] {
					t.Errorf("Rule %d has invalid action '%s' for %s", i, rule.Action, test.name)
				}

				// Check that CEL expressions exist
				if rule.Expression == nil {
					t.Errorf("Rule %d has no expression for %s", i, test.name)
				}
			}

			t.Logf("Successfully converted %s: %d robots.txt rules -> %d Anubis rules",
				test.name, len(rules), len(anubisRules))
		})
	}
}

func TestCELExpressionGeneration(t *testing.T) {
	tests := []struct {
		name       string
		robotsPath string
		expected   string
	}{
		{
			name:       "simple path",
			robotsPath: "/admin",
			expected:   `path.startsWith("/admin")`,
		},
		{
			name:       "path with trailing slash",
			robotsPath: "/admin/",
			expected:   `path.startsWith("/admin/")`,
		},
		{
			name:       "wildcard path",
			robotsPath: "/search*",
			expected:   `path.matches("^/search.*")`,
		},
		{
			name:       "complex wildcard",
			robotsPath: "/*/wiki/*?action=*",
			expected:   `path.matches("^/.*/wiki/.*.action=.*")`,
		},
		{
			name:       "question mark wildcard",
			robotsPath: "/file?.txt",
			expected:   `path.matches("^/file.\\.txt")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := buildPathCondition(test.robotsPath)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestRobotsTxtParsing(t *testing.T) {
	robotsTxt := `# Example robots.txt
User-agent: *
Disallow: /admin/
Disallow: /private
Allow: /public

User-agent: Googlebot
Disallow: /search
Crawl-delay: 10

User-agent: BadBot
Disallow: /

# Rogue bots
User-agent: SpamBot
Disallow: /`

	reader := strings.NewReader(robotsTxt)
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse test robots.txt: %v", err)
	}

	expectedRules := 4 // *, Googlebot, BadBot, SpamBot
	if len(rules) != expectedRules {
		t.Errorf("Expected %d rules, got %d", expectedRules, len(rules))
	}

	// Check universal rules
	universalRule := rules[0]
	if universalRule.UserAgent != "*" {
		t.Errorf("Expected universal user agent '*', got %q", universalRule.UserAgent)
	}

	if len(universalRule.Disallows) != 2 {
		t.Errorf("Expected 2 disallows for universal rule, got %d", len(universalRule.Disallows))
	}

	if len(universalRule.Allows) != 1 {
		t.Errorf("Expected 1 allow for universal rule, got %d", len(universalRule.Allows))
	}

	// Check Googlebot rules
	googlebotRule := rules[1]
	if googlebotRule.UserAgent != "Googlebot" {
		t.Errorf("Expected Googlebot user agent, got %q", googlebotRule.UserAgent)
	}

	if googlebotRule.CrawlDelay != 10 {
		t.Errorf("Expected crawl delay 10, got %d", googlebotRule.CrawlDelay)
	}

	// Check blacklisted bots
	badBotRule := rules[2]
	if !badBotRule.IsBlacklist {
		t.Errorf("BadBot should be marked as blacklisted")
	}

	spamBotRule := rules[3]
	if !spamBotRule.IsBlacklist {
		t.Errorf("SpamBot should be marked as blacklisted")
	}
}

func TestAnubisRuleGeneration(t *testing.T) {
	// Test with simple robots.txt
	robotsRules := []RobotsRule{
		{
			UserAgent: "*",
			Disallows: []string{"/admin", "/private"},
			Allows:    []string{"/public"},
		},
		{
			UserAgent:   "BadBot",
			Disallows:   []string{"/"},
			IsBlacklist: true,
		},
	}

	oldPolicyName := *policyName
	*policyName = "test-policy"
	defer func() { *policyName = oldPolicyName }()

	oldBaseAction := *baseAction
	*baseAction = "CHALLENGE"
	defer func() { *baseAction = oldBaseAction }()

	oldUserAgentDeny := *userAgentDeny
	*userAgentDeny = "DENY"
	defer func() { *userAgentDeny = oldUserAgentDeny }()

	anubisRules := convertToAnubisRules(robotsRules)

	// Should have 3 rules: 2 disallows + 1 blacklist
	expectedRules := 3
	if len(anubisRules) != expectedRules {
		t.Errorf("Expected %d Anubis rules, got %d", expectedRules, len(anubisRules))
	}

	// Check first disallow rule
	firstRule := anubisRules[0]
	if firstRule.Action != "CHALLENGE" {
		t.Errorf("Expected CHALLENGE action, got %s", firstRule.Action)
	}

	if !strings.Contains(firstRule.Name, "disallow") {
		t.Errorf("Expected disallow in rule name, got %s", firstRule.Name)
	}

	// Check blacklist rule
	var blacklistRule *AnubisRule
	for _, rule := range anubisRules {
		if strings.Contains(rule.Name, "blacklist") {
			blacklistRule = &rule
			break
		}
	}

	if blacklistRule == nil {
		t.Errorf("No blacklist rule found")
	} else {
		if blacklistRule.Action != "DENY" {
			t.Errorf("Expected DENY action for blacklist, got %s", blacklistRule.Action)
		}
	}
}

func TestEmptyRobotsTxt(t *testing.T) {
	reader := strings.NewReader("")
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse empty robots.txt: %v", err)
	}

	if len(rules) != 0 {
		t.Errorf("Expected 0 rules for empty robots.txt, got %d", len(rules))
	}

	anubisRules := convertToAnubisRules(rules)
	if len(anubisRules) != 0 {
		t.Errorf("Expected 0 Anubis rules for empty robots.txt, got %d", len(anubisRules))
	}
}

func TestCommentsOnlyRobotsTxt(t *testing.T) {
	robotsTxt := `# This is a comment
# Another comment
# User-agent: * (commented out)
# Disallow: / (commented out)`

	reader := strings.NewReader(robotsTxt)
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse comments-only robots.txt: %v", err)
	}

	if len(rules) != 0 {
		t.Errorf("Expected 0 rules for comments-only robots.txt, got %d", len(rules))
	}
}

func TestMalformedRobotsTxt(t *testing.T) {
	robotsTxt := `User-agent: *
Disallow /admin (missing colon)
Allow: /public
Random line without colon
User-agent
Disallow: /test`

	reader := strings.NewReader(robotsTxt)
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse malformed robots.txt: %v", err)
	}

	// Should still parse the valid parts
	if len(rules) == 0 {
		t.Errorf("Expected some rules despite malformed input, got 0")
	}

	// Should have at least the Allow rule
	foundAllow := false
	for _, rule := range rules {
		if len(rule.Allows) > 0 {
			foundAllow = true
			break
		}
	}

	if !foundAllow {
		t.Errorf("Expected to find Allow rule in malformed robots.txt")
	}
}
