package policy

import (
	"math"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode/utf8"
)

const maxBranches = 256

type matcher interface {
	MatchString(s string) bool
}

type anyOf struct {
	literals []string
	regexps  []*regexp.Regexp
	minLen   int
}

func (a anyOf) MatchString(s string) bool {
	if len(s) < a.minLen {
		return false
	}

	for _, lit := range a.literals {
		if strings.Contains(s, lit) {
			return true
		}
	}

	for _, rex := range a.regexps {
		if rex.MatchString(s) {
			return true
		}
	}

	return false
}

// splitAlternation returns a matcher that tries each branch of rex on its own
// if every branch starts with literal text, and rex otherwise. Go's regexp
// skips ahead to a literal start with strings.Index, but an alternation like
// "GPTBot|ClaudeBot" has none.
func splitAlternation(rex *regexp.Regexp) matcher {
	tree, err := syntax.Parse(rex.String(), syntax.Perl)
	if err != nil {
		return rex
	}

	alts := branches(tree)
	if len(alts) < 2 {
		return rex
	}

	result := anyOf{minLen: math.MaxInt}
	for _, alt := range alts {
		expr := alt.String()
		if lit, ok := literal(expr); ok {
			result.literals = append(result.literals, lit)
			result.minLen = min(result.minLen, len(lit))
			continue
		}

		branch, err := regexp.Compile(expr)
		if err != nil {
			return rex
		}

		prefix, _ := branch.LiteralPrefix()
		if prefix == "" {
			return rex
		}
		result.regexps = append(result.regexps, branch)
		result.minLen = min(result.minLen, len(prefix))
	}

	return result
}

func literal(expr string) (string, bool) {
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil || re.Op != syntax.OpLiteral || re.Flags&syntax.FoldCase != 0 {
		return "", false
	}

	// U+FFFD also matches invalid UTF-8 in regexp, but not in strings.Contains
	if slices.Contains(re.Rune, utf8.RuneError) {
		return "", false
	}

	return string(re.Rune), true
}

// branches returns the branches of an alternation, undoing the common prefixes
// the parser factors out: a|ab parses as a(?:|b), which gives a and ab. Only
// alternations in a row, like (a|b)(c|d), multiply the count, so only they are
// limited to maxBranches.
func branches(re *syntax.Regexp) []*syntax.Regexp {
	switch re.Op {
	case syntax.OpAlternate:
		var result []*syntax.Regexp
		for _, sub := range re.Sub {
			result = append(result, branches(sub)...)
		}
		return result
	case syntax.OpCapture:
		return branches(re.Sub[0])
	case syntax.OpConcat:
		result := []*syntax.Regexp{{Op: syntax.OpEmptyMatch}}
		for _, sub := range re.Sub {
			alts := branches(sub)
			if len(result) > 1 && len(alts) > 1 && len(result)*len(alts) > maxBranches {
				return []*syntax.Regexp{re}
			}

			next := make([]*syntax.Regexp, 0, len(result)*len(alts))
			for _, prefix := range result {
				for _, alt := range alts {
					next = append(next, concat(prefix, alt))
				}
			}
			result = next
		}
		return result
	}

	return []*syntax.Regexp{re}
}

func concat(a, b *syntax.Regexp) *syntax.Regexp {
	switch {
	case a.Op == syntax.OpEmptyMatch:
		return b
	case b.Op == syntax.OpEmptyMatch:
		return a
	}

	return &syntax.Regexp{Op: syntax.OpConcat, Sub: []*syntax.Regexp{a, b}}
}
