package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type TestCase struct {
	name         string
	robotsFile   string
	expectedFile string
	options      TestOptions
}

type TestOptions struct {
	format           string
	action           string
	policyName       string
	deniedAction     string
	crawlDelayWeight int
}

func TestDataFileConversion(t *testing.T) {

	testCases := []TestCase{
		{
			name:         "simple_default",
			robotsFile:   "simple.robots.txt",
			expectedFile: "simple.yaml",
			options:      TestOptions{format: "yaml"},
		},
		{
			name:         "simple_json",
			robotsFile:   "simple.robots.txt",
			expectedFile: "simple.json",
			options:      TestOptions{format: "json"},
		},
		{
			name:         "simple_deny_action",
			robotsFile:   "simple.robots.txt",
			expectedFile: "deny-action.yaml",
			options:      TestOptions{format: "yaml", action: "DENY"},
		},
		{
			name:         "simple_custom_name",
			robotsFile:   "simple.robots.txt",
			expectedFile: "custom-name.yaml",
			options:      TestOptions{format: "yaml", policyName: "my-custom-policy"},
		},
		{
			name:         "blacklist_with_crawl_delay",
			robotsFile:   "blacklist.robots.txt",
			expectedFile: "blacklist.yaml",
			options:      TestOptions{format: "yaml", crawlDelayWeight: 3},
		},
		{
			name:         "wildcards",
			robotsFile:   "wildcards.robots.txt",
			expectedFile: "wildcards.yaml",
			options:      TestOptions{format: "yaml"},
		},
		{
			name:         "empty_file",
			robotsFile:   "empty.robots.txt",
			expectedFile: "empty.yaml",
			options:      TestOptions{format: "yaml"},
		},
		{
			name:         "complex_scenario",
			robotsFile:   "complex.robots.txt",
			expectedFile: "complex.yaml",
			options:      TestOptions{format: "yaml", crawlDelayWeight: 5},
		},
		{
			name:         "consecutive_user_agents",
			robotsFile:   "consecutive.robots.txt",
			expectedFile: "consecutive.yaml",
			options:      TestOptions{format: "yaml", crawlDelayWeight: 3},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			robotsPath := filepath.Join("testdata", tc.robotsFile)
			expectedPath := filepath.Join("testdata", tc.expectedFile)

			// Read robots.txt input
			robotsFile, err := os.Open(robotsPath)
			if err != nil {
				t.Fatalf("Failed to open robots file %s: %v", robotsPath, err)
			}
			defer robotsFile.Close()

			// Parse robots.txt
			rules, err := parseRobotsTxt(robotsFile)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt: %v", err)
			}

			// Set test options
			oldFormat := *outputFormat
			oldAction := *baseAction
			oldCrawlDelay := *crawlDelay
			oldPolicyName := *policyName
			oldDeniedAction := *userAgentDeny

			if tc.options.format != "" {
				*outputFormat = tc.options.format
			}
			if tc.options.action != "" {
				*baseAction = tc.options.action
			}
			if tc.options.crawlDelayWeight > 0 {
				*crawlDelay = tc.options.crawlDelayWeight
			}
			if tc.options.policyName != "" {
				*policyName = tc.options.policyName
			}
			if tc.options.deniedAction != "" {
				*userAgentDeny = tc.options.deniedAction
			}

			// Restore options after test
			defer func() {
				*outputFormat = oldFormat
				*baseAction = oldAction
				*crawlDelay = oldCrawlDelay
				*policyName = oldPolicyName
				*userAgentDeny = oldDeniedAction
			}()

			// Convert to Anubis rules
			anubisRules := convertToAnubisRules(rules)

			// Generate output
			var actualOutput []byte
			switch strings.ToLower(*outputFormat) {
			case "yaml":
				actualOutput, err = yaml.Marshal(anubisRules)
			case "json":
				actualOutput, err = json.MarshalIndent(anubisRules, "", "  ")
			}
			if err != nil {
				t.Fatalf("Failed to marshal output: %v", err)
			}

			// Read expected output
			expectedOutput, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("Failed to read expected file %s: %v", expectedPath, err)
			}

			if strings.ToLower(*outputFormat) == "yaml" {
				var actualData []interface{}
				var expectedData []interface{}

				err = yaml.Unmarshal(actualOutput, &actualData)
				if err != nil {
					t.Fatalf("Failed to unmarshal actual output: %v", err)
				}

				err = yaml.Unmarshal(expectedOutput, &expectedData)
				if err != nil {
					t.Fatalf("Failed to unmarshal expected output: %v", err)
				}

				// Compare data structures
				if !compareData(actualData, expectedData) {
					actualStr := strings.TrimSpace(string(actualOutput))
					expectedStr := strings.TrimSpace(string(expectedOutput))
					t.Errorf("Output mismatch for %s\nExpected:\n%s\n\nActual:\n%s", tc.name, expectedStr, actualStr)
				}
			} else {
				var actualData []interface{}
				var expectedData []interface{}

				err = json.Unmarshal(actualOutput, &actualData)
				if err != nil {
					t.Fatalf("Failed to unmarshal actual JSON output: %v", err)
				}

				err = json.Unmarshal(expectedOutput, &expectedData)
				if err != nil {
					t.Fatalf("Failed to unmarshal expected JSON output: %v", err)
				}

				// Compare data structures
				if !compareData(actualData, expectedData) {
					actualStr := strings.TrimSpace(string(actualOutput))
					expectedStr := strings.TrimSpace(string(expectedOutput))
					t.Errorf("Output mismatch for %s\nExpected:\n%s\n\nActual:\n%s", tc.name, expectedStr, actualStr)
				}
			}
		})
	}
}

func TestCaseInsensitiveParsing(t *testing.T) {
	robotsTxt := `User-Agent: *
Disallow: /admin
Crawl-Delay: 10

User-agent: TestBot
disallow: /test
crawl-delay: 5

USER-AGENT: UpperBot
DISALLOW: /upper
CRAWL-DELAY: 20`

	reader := strings.NewReader(robotsTxt)
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse case-insensitive robots.txt: %v", err)
	}

	expectedRules := 3
	if len(rules) != expectedRules {
		t.Errorf("Expected %d rules, got %d", expectedRules, len(rules))
	}

	// Check that all crawl delays were parsed
	for i, rule := range rules {
		expectedDelays := []int{10, 5, 20}
		if rule.CrawlDelay != expectedDelays[i] {
			t.Errorf("Rule %d: expected crawl delay %d, got %d", i, expectedDelays[i], rule.CrawlDelay)
		}
	}
}

func TestVariousOutputFormats(t *testing.T) {
	robotsTxt := `User-agent: *
Disallow: /admin`

	reader := strings.NewReader(robotsTxt)
	rules, err := parseRobotsTxt(reader)
	if err != nil {
		t.Fatalf("Failed to parse robots.txt: %v", err)
	}

	oldPolicyName := *policyName
	*policyName = "test-policy"
	defer func() { *policyName = oldPolicyName }()

	anubisRules := convertToAnubisRules(rules)

	// Test YAML output
	yamlOutput, err := yaml.Marshal(anubisRules)
	if err != nil {
		t.Fatalf("Failed to marshal YAML: %v", err)
	}

	if !strings.Contains(string(yamlOutput), "name: test-policy-disallow-1") {
		t.Errorf("YAML output doesn't contain expected rule name")
	}

	// Test JSON output
	jsonOutput, err := json.MarshalIndent(anubisRules, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if !strings.Contains(string(jsonOutput), `"name": "test-policy-disallow-1"`) {
		t.Errorf("JSON output doesn't contain expected rule name")
	}
}

func TestDifferentActions(t *testing.T) {
	robotsTxt := `User-agent: *
Disallow: /admin`

	testActions := []string{"ALLOW", "DENY", "CHALLENGE", "WEIGH"}

	for _, action := range testActions {
		t.Run("action_"+action, func(t *testing.T) {
			reader := strings.NewReader(robotsTxt)
			rules, err := parseRobotsTxt(reader)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt: %v", err)
			}

			oldAction := *baseAction
			*baseAction = action
			defer func() { *baseAction = oldAction }()

			anubisRules := convertToAnubisRules(rules)

			if len(anubisRules) != 1 {
				t.Fatalf("Expected 1 rule, got %d", len(anubisRules))
			}

			if anubisRules[0].Action != action {
				t.Errorf("Expected action %s, got %s", action, anubisRules[0].Action)
			}
		})
	}
}

func TestPolicyNaming(t *testing.T) {
	robotsTxt := `User-agent: *
Disallow: /admin
Disallow: /private

User-agent: BadBot
Disallow: /`

	testNames := []string{"custom-policy", "my-rules", "site-protection"}

	for _, name := range testNames {
		t.Run("name_"+name, func(t *testing.T) {
			reader := strings.NewReader(robotsTxt)
			rules, err := parseRobotsTxt(reader)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt: %v", err)
			}

			oldName := *policyName
			*policyName = name
			defer func() { *policyName = oldName }()

			anubisRules := convertToAnubisRules(rules)

			// Check that all rule names use the custom prefix
			for _, rule := range anubisRules {
				if !strings.HasPrefix(rule.Name, name+"-") {
					t.Errorf("Rule name %s doesn't start with expected prefix %s-", rule.Name, name)
				}
			}
		})
	}
}

func TestCrawlDelayWeights(t *testing.T) {
	robotsTxt := `User-agent: *
Disallow: /admin
Crawl-delay: 10

User-agent: SlowBot
Disallow: /slow
Crawl-delay: 60`

	testWeights := []int{1, 5, 10, 25}

	for _, weight := range testWeights {
		t.Run(fmt.Sprintf("weight_%d", weight), func(t *testing.T) {
			reader := strings.NewReader(robotsTxt)
			rules, err := parseRobotsTxt(reader)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt: %v", err)
			}

			oldWeight := *crawlDelay
			*crawlDelay = weight
			defer func() { *crawlDelay = oldWeight }()

			anubisRules := convertToAnubisRules(rules)

			// Count weight rules and verify they have correct weight
			weightRules := 0
			for _, rule := range anubisRules {
				if rule.Action == "WEIGH" && rule.Weight != nil {
					weightRules++
					if rule.Weight.Adjust != weight {
						t.Errorf("Expected weight %d, got %d", weight, rule.Weight.Adjust)
					}
				}
			}

			expectedWeightRules := 2 // One for *, one for SlowBot
			if weightRules != expectedWeightRules {
				t.Errorf("Expected %d weight rules, got %d", expectedWeightRules, weightRules)
			}
		})
	}
}

func TestBlacklistActions(t *testing.T) {
	robotsTxt := `User-agent: BadBot
Disallow: /

User-agent: SpamBot
Disallow: /`

	testActions := []string{"DENY", "CHALLENGE"}

	for _, action := range testActions {
		t.Run("blacklist_"+action, func(t *testing.T) {
			reader := strings.NewReader(robotsTxt)
			rules, err := parseRobotsTxt(reader)
			if err != nil {
				t.Fatalf("Failed to parse robots.txt: %v", err)
			}

			oldAction := *userAgentDeny
			*userAgentDeny = action
			defer func() { *userAgentDeny = oldAction }()

			anubisRules := convertToAnubisRules(rules)

			// All rules should be blacklist rules with the specified action
			for _, rule := range anubisRules {
				if !strings.Contains(rule.Name, "blacklist") {
					t.Errorf("Expected blacklist rule, got %s", rule.Name)
				}
				if rule.Action != action {
					t.Errorf("Expected action %s, got %s", action, rule.Action)
				}
			}
		})
	}
}

// compareData performs a deep comparison of two data structures,
// ignoring differences that are semantically equivalent in YAML/JSON
func compareData(actual, expected interface{}) bool {
	return reflect.DeepEqual(actual, expected)
}
