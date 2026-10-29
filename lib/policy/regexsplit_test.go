package policy

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var splitAlternationTests = []struct {
	name     string
	rexStr   string
	split    bool
	literals []string
}{
	{
		name:     "literals",
		rexStr:   "GPTBot|ClaudeBot",
		split:    true,
		literals: []string{"GPTBot", "ClaudeBot"},
	},
	{
		name:     "common_prefix",
		rexStr:   "GoogleOther|GoogleOther-Image|GoogleOther-Video",
		split:    true,
		literals: []string{"GoogleOther", "GoogleOther-Image", "GoogleOther-Video"},
	},
	{
		name:     "wildcard_after_prefix",
		rexStr:   "cohere-ai|cohere-training-data-crawler|Brightbot 1.0",
		split:    true,
		literals: []string{"cohere-ai", "cohere-training-data-crawler"},
	},
	{
		name:   "flags_in_branch",
		rexStr: "GPT(?i:bot)|Claude(?i:bot)",
		split:  true,
	},
	{
		name:     "group",
		rexStr:   "a(bc|de)f|x",
		split:    true,
		literals: []string{"abcf", "adef", "x"},
	},
	{
		name:     "anchor",
		rexStr:   "^/api|/static/",
		split:    true,
		literals: []string{"/static/"},
	},
	{
		name:     "replacement_character",
		rexStr:   `a\x{FFFD}|GPTBot`,
		split:    true,
		literals: []string{"GPTBot"},
	},
	{
		name:     "many_branches",
		rexStr:   strings.Join(botNames(2*maxBranches), "|"),
		split:    true,
		literals: botNames(2 * maxBranches),
	},
	{
		name:   "case_insensitive",
		rexStr: "(?i)gptbot|claudebot",
		split:  false,
	},
	{
		name:   "no_literal_prefix",
		rexStr: "GPTBot|.*bot",
		split:  false,
	},
	{
		name:   "empty_branch",
		rexStr: "GPTBot|",
		split:  false,
	},
	{
		name:   "too_many_branches",
		rexStr: "(ab|cd)(ab|cd)(ab|cd)(ab|cd)(ab|cd)(ab|cd)(ab|cd)(ab|cd)(ab|cd)",
		split:  false,
	},
	{
		name:   "not_alternation",
		rexStr: "GPTBot",
		split:  false,
	},
}

func botNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("Bot%d/", i)
	}

	return names
}

func TestSplitAlternation(t *testing.T) {
	for _, tt := range splitAlternationTests {
		t.Run(tt.name, func(t *testing.T) {
			m := splitAlternation(regexp.MustCompile(tt.rexStr))

			a, split := m.(anyOf)
			if split != tt.split {
				t.Errorf("split: %v, wanted: %v", split, tt.split)
			}

			if !slices.Equal(a.literals, tt.literals) {
				t.Errorf("literals: %q, wanted: %q", a.literals, tt.literals)
			}
		})
	}
}

// FuzzSplitAlternation tests that a split regex matches the same strings as the original
func FuzzSplitAlternation(f *testing.F) {
	inputs := []string{
		"",
		"GPTBo",
		"GPTBot",
		"GPTBOT",
		"gptbot",
		"x ClaudeBot/1.0",
		"GoogleOther-Video",
		"GoogleOthe",
		"cohere-training",
		"Brightbot 1x0",
		"abcf",
		"adef",
		"af",
		"xyz",
		"/api",
		"/api/v1",
		"/x/api",
		"/static/a.css",
		"a\xff",
		"Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
	}

	for _, tt := range splitAlternationTests {
		for _, input := range inputs {
			f.Add(tt.rexStr, input)
		}
	}

	f.Fuzz(func(t *testing.T, rexStr, input string) {
		rex, err := regexp.Compile(rexStr)
		if err != nil {
			t.Skip()
		}

		if got, want := splitAlternation(rex).MatchString(input), rex.MatchString(input); got != want {
			t.Errorf("%q matching %q: %v, wanted: %v", rexStr, input, got, want)
		}
	})
}
