package glob

import "testing"

func TestGlob_EqualityAndEmpty(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		subj    string
		want    bool
	}{
		{"exact match", "hello", "hello", true},
		{"exact mismatch", "hello", "hell", false},
		{"empty pattern and subject", "", "", true},
		{"empty pattern with non-empty subject", "", "x", false},
		{"pattern star matches empty", "*", "", true},
		{"pattern star matches anything", "*", "anything at all", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Glob(tc.pattern, tc.subj); got != tc.want {
				t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
			}
		})
	}
}

func TestGlob_LeadingAndTrailing(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		subj    string
		want    bool
	}{
		{"prefix match - minimal", "foo*", "foo", true},
		{"prefix match - extended", "foo*", "foobar", true},
		{"prefix mismatch - not at start", "foo*", "xfoo", false},
		{"suffix match - minimal", "*foo", "foo", true},
		{"suffix match - extended", "*foo", "xfoo", true},
		{"suffix mismatch - not at end", "*foo", "foox", false},
		{"contains match", "*foo*", "barfoobaz", true},
		{"contains mismatch - missing needle", "*foo*", "f", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Glob(tc.pattern, tc.subj); got != tc.want {
				t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
			}
		})
	}
}

func TestGlob_MiddleAndOrder(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		subj    string
		want    bool
	}{
		{"middle wildcard basic", "f*o", "fo", true},
		{"middle wildcard gap", "f*o", "fZZZo", true},
		{"middle wildcard requires start f", "f*o", "xfyo", false},
		{"order enforced across parts", "a*b*c*d", "axxbxxcxxd", true},
		{"order mismatch fails", "a*b*c*d", "abdc", false},
		{"must end with last part when no trailing *", "*foo*bar", "zzfooqqbar", true},
		{"failing when trailing chars remain", "*foo*bar", "zzfooqqbarzz", false},
		{"first part must start when no leading *", "foo*bar", "zzfooqqbar", false},
		{"works with overlapping content", "ab*ba", "ababa", true},
		{"needle not found fails", "foo*bar", "foobaz", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Glob(tc.pattern, tc.subj); got != tc.want {
				t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
			}
		})
	}
}

func TestGlob_ConsecutiveStarsAndEmptyParts(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		subj    string
		want    bool
	}{
		{"double star matches anything", "**", "", true},
		{"double star matches anything non-empty", "**", "abc", true},
		{"consecutive stars behave like single", "a**b", "ab", true},
		{"consecutive stars with gaps", "a**b", "axxxb", true},
		{"consecutive stars + trailing star", "a**b*", "axxbzzz", true},
		{"consecutive stars still enforce anchors", "a**b", "xaBy", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Glob(tc.pattern, tc.subj); got != tc.want {
				t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
			}
		})
	}
}

func TestGlob_MaxPartsLimit(t *testing.T) {
	// Allowed: up to 4 '*' (5 parts)
	allowed := []struct {
		pattern string
		subj    string
		want    bool
	}{
		{"a*b*c*d*e", "axxbxxcxxdxxe", true}, // 4 stars -> 5 parts
		{"*a*b*c*d", "zzzaaaabbbcccddd", true},
		{"a*b*c*d*e", "abcde", true},
		{"a*b*c*d*e", "abxdxe", false}, // missing 'c' should fail
	}
	for _, tc := range allowed {
		if got := Glob(tc.pattern, tc.subj); got != tc.want {
			t.Fatalf("allowed pattern Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
		}
	}

	// Disallowed: 5 '*' (6 parts) -> always false by complexity check
	disallowed := []struct {
		pattern string
		subj    string
	}{
		{"a*b*c*d*e*f", "aXXbYYcZZdQQeRRf"},
		{"*a*b*c*d*e*", "abcdef"},
		{"******", "anything"}, // 6 stars -> 7 parts
	}
	for _, tc := range disallowed {
		if got := Glob(tc.pattern, tc.subj); got {
			t.Fatalf("disallowed pattern should fail Glob(%q,%q) = %v, want false", tc.pattern, tc.subj, got)
		}
	}
}

func TestGlob_CaseSensitivity(t *testing.T) {
	cases := []struct {
		pattern string
		subj    string
		want    bool
	}{
		{"FOO*", "foo", false},
		{"*Bar", "bar", false},
		{"Foo*Bar", "FooZZZBar", true},
	}
	for _, tc := range cases {
		if got := Glob(tc.pattern, tc.subj); got != tc.want {
			t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
		}
	}
}

func TestGlob_EmptySubjectInteractions(t *testing.T) {
	cases := []struct {
		pattern string
		subj    string
		want    bool
	}{
		{"*a", "", false},
		{"a*", "", false},
		{"**", "", true},
		{"*", "", true},
	}
	for _, tc := range cases {
		if got := Glob(tc.pattern, tc.subj); got != tc.want {
			t.Fatalf("Glob(%q,%q) = %v, want %v", tc.pattern, tc.subj, got, tc.want)
		}
	}
}

func BenchmarkGlob(b *testing.B) {
	patterns := []string{
		"*", "*foo*", "foo*bar", "a*b*c*d*e", "a**b*", "*needle*end",
	}
	subjects := []string{
		"", "foo", "barfoo", "foobarbaz", "axxbxxcxxdxxe", "zzfooqqbarzz",
		"lorem ipsum dolor sit amet, consectetur adipiscing elit",
	}
	for _, p := range patterns {
		for _, s := range subjects {
			b.Run(p+"::"+s, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = Glob(p, s)
				}
			})
		}
	}
}
