package expressions

import (
	"testing"

	"github.com/google/cel-go/common/types"
)

func TestBotEnvironment(t *testing.T) {
	env, err := BotEnvironment()
	if err != nil {
		t.Fatalf("failed to create bot environment: %v", err)
	}

	t.Run("missingHeader", func(t *testing.T) {
		tests := []struct {
			headers     map[string]string
			name        string
			expression  string
			description string
			expected    types.Bool
		}{
			{
				name:       "missing-header",
				expression: `missingHeader(headers, "Missing-Header")`,
				headers: map[string]string{
					"User-Agent":   "test-agent",
					"Content-Type": "application/json",
				},
				expected:    types.Bool(true),
				description: "should return true when header is missing",
			},
			{
				name:       "existing-header",
				expression: `missingHeader(headers, "User-Agent")`,
				headers: map[string]string{
					"User-Agent":   "test-agent",
					"Content-Type": "application/json",
				},
				expected:    types.Bool(false),
				description: "should return false when header exists",
			},
			{
				name:       "case-sensitive",
				expression: `missingHeader(headers, "user-agent")`,
				headers: map[string]string{
					"User-Agent": "test-agent",
				},
				expected:    types.Bool(true),
				description: "should be case-sensitive (user-agent != User-Agent)",
			},
			{
				name:        "empty-headers",
				expression:  `missingHeader(headers, "Any-Header")`,
				headers:     map[string]string{},
				expected:    types.Bool(true),
				description: "should return true for any header when map is empty",
			},
			{
				name:       "real-world-sec-ch-ua",
				expression: `missingHeader(headers, "Sec-Ch-Ua")`,
				headers: map[string]string{
					"User-Agent": "curl/7.68.0",
					"Accept":     "*/*",
					"Host":       "example.com",
				},
				expected:    types.Bool(true),
				description: "should detect missing browser-specific headers from bots",
			},
			{
				name:       "browser-with-sec-ch-ua",
				expression: `missingHeader(headers, "Sec-Ch-Ua")`,
				headers: map[string]string{
					"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
					"Sec-Ch-Ua":  `"Chrome"; v="91", "Not A Brand"; v="99"`,
					"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
				},
				expected:    types.Bool(false),
				description: "should return false when browser sends Sec-Ch-Ua header",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				prog, err := Compile(env, tt.expression)
				if err != nil {
					t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
				}

				result, _, err := prog.Eval(map[string]interface{}{
					"headers": tt.headers,
				})
				if err != nil {
					t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
				}

				if result != tt.expected {
					t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
				}
			})
		}

		t.Run("function-compilation", func(t *testing.T) {
			src := `missingHeader(headers, "Test-Header")`
			_, err := Compile(env, src)
			if err != nil {
				t.Fatalf("failed to compile missingHeader expression: %v", err)
			}
		})
	})

	t.Run("segments", func(t *testing.T) {
		for _, tt := range []struct {
			name        string
			description string
			expression  string
			path        string
			expected    types.Bool
		}{
			{
				name:        "simple",
				description: "/ should have one path segment",
				expression:  `size(segments(path)) == 1`,
				path:        "/",
				expected:    types.Bool(true),
			},
			{
				name:        "two segments without trailing slash",
				description: "/user/foo should have two segments",
				expression:  `size(segments(path)) == 2`,
				path:        "/user/foo",
				expected:    types.Bool(true),
			},
			{
				name:        "at least two segments",
				description: "/foo/bar/ should have at least two path segments",
				expression:  `size(segments(path)) >= 2`,
				path:        "/foo/bar/",
				expected:    types.Bool(true),
			},
			{
				name:        "at most two segments",
				description: "/foo/bar/ does not have less than two path segments",
				expression:  `size(segments(path)) < 2`,
				path:        "/foo/bar/",
				expected:    types.Bool(false),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				prog, err := Compile(env, tt.expression)
				if err != nil {
					t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
				}

				result, _, err := prog.Eval(map[string]interface{}{
					"path": tt.path,
				})
				if err != nil {
					t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
				}

				if result != tt.expected {
					t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
				}
			})
		}

		t.Run("invalid", func(t *testing.T) {
			for _, tt := range []struct {
				env             any
				name            string
				description     string
				expression      string
				wantFailCompile bool
				wantFailEval    bool
			}{
				{
					name:        "segments of headers",
					description: "headers are not a path list",
					expression:  `segments(headers)`,
					env: map[string]any{
						"headers": map[string]string{
							"foo": "bar",
						},
					},
					wantFailCompile: true,
				},
				{
					name:        "invalid path type",
					description: "a path should be a sting",
					expression:  `size(segments(path)) != 0`,
					env: map[string]any{
						"path": 4,
					},
					wantFailEval: true,
				},
				{
					name:        "invalid path",
					description: "a path should start with a leading slash",
					expression:  `size(segments(path)) != 0`,
					env: map[string]any{
						"path": "foo",
					},
					wantFailEval: true,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					prog, err := Compile(env, tt.expression)
					if err != nil {
						if !tt.wantFailCompile {
							t.Log(tt.description)
							t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
						} else {
							return
						}
					}

					_, _, err = prog.Eval(tt.env)

					if err == nil {
						t.Log(tt.description)
						t.Fatal("wanted an error but got none")
					}

					t.Log(err)
				})
			}
		})

		t.Run("function-compilation", func(t *testing.T) {
			src := `size(segments(path)) <= 2`
			_, err := Compile(env, src)
			if err != nil {
				t.Fatalf("failed to compile missingHeader expression: %v", err)
			}
		})
	})
}

func TestThresholdEnvironment(t *testing.T) {
	env, err := ThresholdEnvironment()
	if err != nil {
		t.Fatalf("failed to create threshold environment: %v", err)
	}

	tests := []struct {
		variables     map[string]interface{}
		name          string
		expression    string
		description   string
		expected      types.Bool
		shouldCompile bool
	}{
		{
			name:          "weight-variable-available",
			expression:    `weight > 100`,
			variables:     map[string]interface{}{"weight": 150},
			expected:      types.Bool(true),
			description:   "should support weight variable in expressions",
			shouldCompile: true,
		},
		{
			name:          "weight-variable-false-case",
			expression:    `weight > 100`,
			variables:     map[string]interface{}{"weight": 50},
			expected:      types.Bool(false),
			description:   "should correctly evaluate weight comparisons",
			shouldCompile: true,
		},
		{
			name:          "missingHeader-not-available",
			expression:    `missingHeader(headers, "Test")`,
			variables:     map[string]interface{}{},
			expected:      types.Bool(false), // not used
			description:   "should not have missingHeader function available",
			shouldCompile: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := Compile(env, tt.expression)

			if !tt.shouldCompile {
				if err == nil {
					t.Fatalf("%s: expected compilation to fail but it succeeded", tt.description)
				}
				return // Test passed - compilation failed as expected
			}

			if err != nil {
				t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
			}

			result, _, err := prog.Eval(tt.variables)
			if err != nil {
				t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
			}

			if result != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
			}
		})
	}
}

func TestNewEnvironment(t *testing.T) {
	env, err := New()
	if err != nil {
		t.Fatalf("failed to create new environment: %v", err)
	}

	tests := []struct {
		name          string
		expression    string
		variables     map[string]interface{}
		expectBool    *bool // nil if we just want to test compilation or non-bool result
		description   string
		shouldCompile bool
	}{
		{
			name:          "randInt-function-compilation",
			expression:    `randInt(10)`,
			variables:     map[string]interface{}{},
			expectBool:    nil, // Don't check result, just compilation
			description:   "should compile randInt function",
			shouldCompile: true,
		},
		{
			name:          "randInt-range-validation",
			expression:    `randInt(10) >= 0 && randInt(10) < 10`,
			variables:     map[string]interface{}{},
			expectBool:    boolPtr(true),
			description:   "should return values in correct range",
			shouldCompile: true,
		},
		{
			name:          "strings-extension-size",
			expression:    `"hello".size() == 5`,
			variables:     map[string]interface{}{},
			expectBool:    boolPtr(true),
			description:   "should support string extension functions",
			shouldCompile: true,
		},
		{
			name:          "strings-extension-contains",
			expression:    `"hello world".contains("world")`,
			variables:     map[string]interface{}{},
			expectBool:    boolPtr(true),
			description:   "should support string contains function",
			shouldCompile: true,
		},
		{
			name:          "strings-extension-startsWith",
			expression:    `"hello world".startsWith("hello")`,
			variables:     map[string]interface{}{},
			expectBool:    boolPtr(true),
			description:   "should support string startsWith function",
			shouldCompile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, err := Compile(env, tt.expression)

			if !tt.shouldCompile {
				if err == nil {
					t.Fatalf("%s: expected compilation to fail but it succeeded", tt.description)
				}
				return // Test passed - compilation failed as expected
			}

			if err != nil {
				t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
			}

			// If we only want to test compilation, skip evaluation
			if tt.expectBool == nil {
				return
			}

			result, _, err := prog.Eval(tt.variables)
			if err != nil {
				t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
			}

			if result != types.Bool(*tt.expectBool) {
				t.Errorf("%s: expected %v, got %v", tt.description, *tt.expectBool, result)
			}
		})
	}
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}
