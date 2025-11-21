package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/lib/config"

	"sigs.k8s.io/yaml"
)

var (
	inputFile     = flag.String("input", "", "path to robots.txt file (use - for stdin)")
	outputFile    = flag.String("output", "", "output file path (use - for stdout, defaults to stdout)")
	outputFormat  = flag.String("format", "yaml", "output format: yaml or json")
	baseAction    = flag.String("action", "CHALLENGE", "default action for disallowed paths: ALLOW, DENY, CHALLENGE, WEIGH")
	crawlDelay    = flag.Int("crawl-delay-weight", 0, "if > 0, add weight adjustment for crawl-delay (difficulty adjustment)")
	policyName    = flag.String("name", "robots-txt-policy", "name for the generated policy")
	userAgentDeny = flag.String("deny-user-agents", "DENY", "action for specifically blocked user agents: DENY, CHALLENGE")
	helpFlag      = flag.Bool("help", false, "show help")
)

type RobotsRule struct {
	UserAgents  []string
	Disallows   []string
	Allows      []string
	CrawlDelay  int
	IsBlacklist bool // true if this is a specifically denied user agent
}

type AnubisRule struct {
	Expression *config.ExpressionOrList `yaml:"expression,omitempty" json:"expression,omitempty"`
	Challenge  *config.ChallengeRules   `yaml:"challenge,omitempty" json:"challenge,omitempty"`
	Weight     *config.Weight           `yaml:"weight,omitempty" json:"weight,omitempty"`
	Name       string                   `yaml:"name" json:"name"`
	Action     string                   `yaml:"action" json:"action"`
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s [options] -input <robots.txt>\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  # Convert local robots.txt file")
		fmt.Fprintln(os.Stderr, "  robots2policy -input robots.txt -output policy.yaml")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Convert from URL")
		fmt.Fprintln(os.Stderr, "  robots2policy -input https://example.com/robots.txt -format json")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Read from stdin, write to stdout")
		fmt.Fprintln(os.Stderr, "  curl https://example.com/robots.txt | robots2policy -input -")
		os.Exit(2)
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) > 0 || *helpFlag || *inputFile == "" {
		flag.Usage()
	}

	// Read robots.txt
	var input io.Reader
	if *inputFile == "-" {
		input = os.Stdin
	} else if strings.HasPrefix(*inputFile, "http://") || strings.HasPrefix(*inputFile, "https://") {
		resp, err := http.Get(*inputFile)
		if err != nil {
			log.Fatalf("failed to fetch robots.txt from URL: %v", err)
		}
		defer resp.Body.Close()
		input = resp.Body
	} else {
		file, err := os.Open(*inputFile)
		if err != nil {
			log.Fatalf("failed to open input file: %v", err)
		}
		defer file.Close()
		input = file
	}

	// Parse robots.txt
	rules, err := parseRobotsTxt(input)
	if err != nil {
		log.Fatalf("failed to parse robots.txt: %v", err)
	}

	// Convert to Anubis rules
	anubisRules := convertToAnubisRules(rules)

	// Check if any rules were generated
	if len(anubisRules) == 0 {
		log.Fatal("no valid rules generated from robots.txt - file may be empty or contain no disallow directives")
	}

	// Generate output
	var output []byte
	switch strings.ToLower(*outputFormat) {
	case "yaml":
		output, err = yaml.Marshal(anubisRules)
	case "json":
		output, err = json.MarshalIndent(anubisRules, "", "  ")
	default:
		log.Fatalf("unsupported output format: %s (use yaml or json)", *outputFormat)
	}

	if err != nil {
		log.Fatalf("failed to marshal output: %v", err)
	}

	// Write output
	if *outputFile == "" || *outputFile == "-" {
		fmt.Print(string(output))
	} else {
		err = os.WriteFile(*outputFile, output, 0644)
		if err != nil {
			log.Fatalf("failed to write output file: %v", err)
		}
		fmt.Printf("Generated Anubis policy written to %s\n", *outputFile)
	}
}

func createRuleFromAccumulated(userAgents, disallows, allows []string, crawlDelay int) RobotsRule {
	rule := RobotsRule{
		UserAgents: make([]string, len(userAgents)),
		Disallows:  make([]string, len(disallows)),
		Allows:     make([]string, len(allows)),
		CrawlDelay: crawlDelay,
	}
	copy(rule.UserAgents, userAgents)
	copy(rule.Disallows, disallows)
	copy(rule.Allows, allows)
	return rule
}

func parseRobotsTxt(input io.Reader) ([]RobotsRule, error) {
	scanner := bufio.NewScanner(input)
	var rules []RobotsRule
	var currentUserAgents []string
	var currentDisallows []string
	var currentAllows []string
	var currentCrawlDelay int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first colon
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "user-agent":
			// If we have accumulated rules with directives and encounter a new user-agent,
			// flush the current rules
			if len(currentUserAgents) > 0 && (len(currentDisallows) > 0 || len(currentAllows) > 0 || currentCrawlDelay > 0) {
				rule := createRuleFromAccumulated(currentUserAgents, currentDisallows, currentAllows, currentCrawlDelay)
				rules = append(rules, rule)
				// Reset for next group
				currentUserAgents = nil
				currentDisallows = nil
				currentAllows = nil
				currentCrawlDelay = 0
			}
			currentUserAgents = append(currentUserAgents, value)

		case "disallow":
			if len(currentUserAgents) > 0 && value != "" {
				currentDisallows = append(currentDisallows, value)
			}

		case "allow":
			if len(currentUserAgents) > 0 && value != "" {
				currentAllows = append(currentAllows, value)
			}

		case "crawl-delay":
			if len(currentUserAgents) > 0 {
				if delay, err := parseIntSafe(value); err == nil {
					currentCrawlDelay = delay
				}
			}
		}
	}

	// Don't forget the last group of rules
	if len(currentUserAgents) > 0 {
		rule := createRuleFromAccumulated(currentUserAgents, currentDisallows, currentAllows, currentCrawlDelay)
		rules = append(rules, rule)
	}

	// Mark blacklisted user agents (those with "Disallow: /")
	for i := range rules {
		for _, disallow := range rules[i].Disallows {
			if disallow == "/" {
				rules[i].IsBlacklist = true
				break
			}
		}
	}

	return rules, scanner.Err()
}

func parseIntSafe(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func convertToAnubisRules(robotsRules []RobotsRule) []AnubisRule {
	var anubisRules []AnubisRule
	ruleCounter := 0

	// Process each robots rule individually
	for _, robotsRule := range robotsRules {
		userAgents := robotsRule.UserAgents

		// Handle crawl delay
		if robotsRule.CrawlDelay > 0 && *crawlDelay > 0 {
			ruleCounter++
			rule := AnubisRule{
				Name:   fmt.Sprintf("%s-crawl-delay-%d", *policyName, ruleCounter),
				Action: "WEIGH",
				Weight: &config.Weight{Adjust: *crawlDelay},
			}

			if len(userAgents) == 1 && userAgents[0] == "*" {
				rule.Expression = &config.ExpressionOrList{
					All: []string{"true"}, // Always applies
				}
			} else if len(userAgents) == 1 {
				rule.Expression = &config.ExpressionOrList{
					All: []string{fmt.Sprintf("userAgent.contains(%q)", userAgents[0])},
				}
			} else {
				// Multiple user agents - use any block
				var expressions []string
				for _, ua := range userAgents {
					if ua == "*" {
						expressions = append(expressions, "true")
					} else {
						expressions = append(expressions, fmt.Sprintf("userAgent.contains(%q)", ua))
					}
				}
				rule.Expression = &config.ExpressionOrList{
					Any: expressions,
				}
			}
			anubisRules = append(anubisRules, rule)
		}

		// Handle blacklisted user agents
		if robotsRule.IsBlacklist {
			ruleCounter++
			rule := AnubisRule{
				Name:   fmt.Sprintf("%s-blacklist-%d", *policyName, ruleCounter),
				Action: *userAgentDeny,
			}

			if len(userAgents) == 1 {
				userAgent := userAgents[0]
				if userAgent == "*" {
					// This would block everything - convert to a weight adjustment instead
					rule.Name = fmt.Sprintf("%s-global-restriction-%d", *policyName, ruleCounter)
					rule.Action = "WEIGH"
					rule.Weight = &config.Weight{Adjust: 20} // Increase difficulty significantly
					rule.Expression = &config.ExpressionOrList{
						All: []string{"true"}, // Always applies
					}
				} else {
					rule.Expression = &config.ExpressionOrList{
						All: []string{fmt.Sprintf("userAgent.contains(%q)", userAgent)},
					}
				}
			} else {
				// Multiple user agents - use any block
				var expressions []string
				for _, ua := range userAgents {
					if ua == "*" {
						expressions = append(expressions, "true")
					} else {
						expressions = append(expressions, fmt.Sprintf("userAgent.contains(%q)", ua))
					}
				}
				rule.Expression = &config.ExpressionOrList{
					Any: expressions,
				}
			}
			anubisRules = append(anubisRules, rule)
		}

		// Handle specific disallow rules
		for _, disallow := range robotsRule.Disallows {
			if disallow == "/" {
				continue // Already handled as blacklist above
			}

			ruleCounter++
			rule := AnubisRule{
				Name:   fmt.Sprintf("%s-disallow-%d", *policyName, ruleCounter),
				Action: *baseAction,
			}

			// Build CEL expression
			var conditions []string

			// Add user agent conditions
			if len(userAgents) == 1 && userAgents[0] == "*" {
				// Wildcard user agent - no user agent condition needed
			} else if len(userAgents) == 1 {
				conditions = append(conditions, fmt.Sprintf("userAgent.contains(%q)", userAgents[0]))
			} else {
				// For multiple user agents, we need to use a more complex expression
				// This is a limitation - we can't easily combine any for user agents with all for path
				// So we'll create separate rules for each user agent
				for _, ua := range userAgents {
					if ua == "*" {
						continue // Skip wildcard as it's handled separately
					}
					ruleCounter++
					subRule := AnubisRule{
						Name:   fmt.Sprintf("%s-disallow-%d", *policyName, ruleCounter),
						Action: *baseAction,
						Expression: &config.ExpressionOrList{
							All: []string{
								fmt.Sprintf("userAgent.contains(%q)", ua),
								buildPathCondition(disallow),
							},
						},
					}
					anubisRules = append(anubisRules, subRule)
				}
				continue
			}

			// Add path condition
			pathCondition := buildPathCondition(disallow)
			conditions = append(conditions, pathCondition)

			rule.Expression = &config.ExpressionOrList{
				All: conditions,
			}

			anubisRules = append(anubisRules, rule)
		}
	}

	return anubisRules
}

func buildPathCondition(robotsPath string) string {
	// Handle wildcards in robots.txt paths
	if strings.Contains(robotsPath, "*") || strings.Contains(robotsPath, "?") {
		// Convert robots.txt wildcards to regex
		regex := regexp.QuoteMeta(robotsPath)
		regex = strings.ReplaceAll(regex, `\*`, `.*`) // * becomes .*
		regex = strings.ReplaceAll(regex, `\?`, `.`)  // ? becomes .
		regex = "^" + regex
		return fmt.Sprintf("path.matches(%q)", regex)
	}

	// Simple prefix match for most cases
	return fmt.Sprintf("path.startsWith(%q)", robotsPath)
}
