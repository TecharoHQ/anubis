package expressions

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func TestBotEnvironment(t *testing.T) {
	dnsObj := internal.NewDNS(300, 300)
	env, err := BotEnvironment(dnsObj)
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

	t.Run("regexSafe", func(t *testing.T) {
		tests := []struct {
			name        string
			expression  string
			expected    types.String
			description string
		}{
			{
				name:        "complex-test",
				expression:  `regexSafe("^(test1|test2|)[a-z]+$")`,
				expected:    types.String("\\^\\(test1\\|test2\\|\\)\\[a\\-z\\]\\+\\$"),
				description: "should escape all reserved regex characters",
			},
			{
				name:        "backslash-test",
				expression:  `regexSafe("use \\\\ for special characters escaping\t, one/\"\\\"/for/cel and one/for/regex")`,
				expected:    types.String("use \\\\\\\\ for special characters escaping\t, one/\"\\\\\"/for/cel and one/for/regex"),
				description: "should escape double-backslashes as double-double-backslashes and ignore cel escaping and forward slashes",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				prog, err := Compile(env, tt.expression)
				if err != nil {
					t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
				}

				result, _, err := prog.Eval(map[string]interface{}{})
				if err != nil {
					t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
				}

				if result != tt.expected {
					t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
				}
			})
		}

		t.Run("function-compilation", func(t *testing.T) {
			src := `regexSafe(".*")`
			_, err := Compile(env, src)
			if err != nil {
				t.Fatalf("failed to compile regexSafe expression: %v", err)
			}
		})
	})

	t.Run("dnsFunctions", func(t *testing.T) {
		originalDNSLookupAddr := internal.DNSLookupAddr
		originalDNSLookupHost := internal.DNSLookupHost
		defer func() {
			internal.DNSLookupAddr = originalDNSLookupAddr
			internal.DNSLookupHost = originalDNSLookupHost
		}()

		t.Run("reverseDNS", func(t *testing.T) {
			tests := []struct {
				name        string
				addr        string
				mockReturn  []string
				mockError   error
				expression  string
				expected    ref.Val
				description string
			}{
				{
					name:        "success",
					addr:        "8.8.8.8",
					mockReturn:  []string{"dns.google."},
					expression:  `reverseDNS("8.8.8.8")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{"dns.google"}),
					description: "should return domain names for an IP",
				},
				{
					name:        "not-found",
					addr:        "127.0.0.1",
					mockReturn:  []string{},
					mockError:   &net.DNSError{IsNotFound: true},
					expression:  `reverseDNS("127.0.0.1")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{}),
					description: "should return an empty list when not found",
				},
				{
					name:        "error",
					addr:        "error-addr",
					mockError:   errors.New("some dns error"),
					expression:  `reverseDNS("error-addr")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{}),
					description: "should return empty list on error",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					internal.DNSLookupAddr = func(addr string) ([]string, error) {
						if addr == tt.addr {
							return tt.mockReturn, tt.mockError
						}
						return nil, errors.New("unexpected address for reverse lookup")
					}

					prog, err := Compile(env, tt.expression)
					if err != nil {
						t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
					}

					result, _, err := prog.Eval(map[string]interface{}{})
					if err != nil {
						t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
					}
					if result.Equal(tt.expected) != types.True {
						t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
					}
				})
			}
		})

		t.Run("lookupHost", func(t *testing.T) {
			tests := []struct {
				name        string
				host        string
				mockReturn  []string
				mockError   error
				expression  string
				expected    ref.Val
				description string
			}{
				{
					name:        "success",
					host:        "dns.google",
					mockReturn:  []string{"8.8.8.8", "8.8.4.4"},
					expression:  `lookupHost("dns.google")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{"8.8.8.8", "8.8.4.4"}),
					description: "should return IPs for a domain name",
				},
				{
					name:        "not-found",
					host:        "nonexistent.domain.example.com",
					mockReturn:  []string{},
					mockError:   &net.DNSError{IsNotFound: true},
					expression:  `lookupHost("nonexistent.domain.example.com")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{}),
					description: "should return an empty list when not found",
				},
				{
					name:        "error",
					host:        "error-host",
					mockError:   errors.New("some dns error"),
					expression:  `lookupHost("error-host")`,
					expected:    types.NewStringList(types.DefaultTypeAdapter, []string{}),
					description: "should return empty list on error",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					internal.DNSLookupHost = func(host string) ([]string, error) {
						if host == tt.host {
							return tt.mockReturn, tt.mockError
						}
						return nil, errors.New("unexpected host for forward lookup")
					}

					prog, err := Compile(env, tt.expression)
					if err != nil {
						t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
					}

					result, _, err := prog.Eval(map[string]interface{}{})
					if err != nil {
						t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
					}
					if result.Equal(tt.expected) != types.True {
						t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
					}
				})
			}
		})

		t.Run("verifyFCrDNS", func(t *testing.T) {
			tests := []struct {
				name              string
				addr              string
				reverseMockReturn []string
				reverseMockError  error
				forwardMockReturn map[string][]string // name -> ips
				forwardMockError  map[string]error
				expression        string
				expected          types.Bool
				description       string
			}{
				{
					name:              "success",
					addr:              "8.8.8.8",
					reverseMockReturn: []string{"dns.google."},
					forwardMockReturn: map[string][]string{"dns.google": {"8.8.8.8", "8.8.4.4"}},
					expression:        `verifyFCrDNS("8.8.8.8")`,
					expected:          types.Bool(true),
					description:       "should return true for valid FCrDNS",
				},
				{
					name:              "failure",
					addr:              "1.2.3.4",
					reverseMockReturn: []string{"spoofed.example.com."},
					forwardMockReturn: map[string][]string{"spoofed.example.com": {"5.6.7.8"}},
					expression:        `verifyFCrDNS("1.2.3.4")`,
					expected:          types.Bool(false),
					description:       "should return false for invalid FCrDNS",
				},
				{
					name:             "reverse-lookup-fails",
					addr:             "1.1.1.1",
					reverseMockError: errors.New("reverse lookup failed"),
					expression:       `verifyFCrDNS("1.1.1.1")`,
					expected:         types.Bool(false),
					description:      "should return false if reverse lookup fails",
				},
				{
					name:              "success-with-pattern",
					addr:              "8.8.8.8",
					reverseMockReturn: []string{"dns.google."},
					forwardMockReturn: map[string][]string{"dns.google": {"8.8.8.8"}},
					expression:        `verifyFCrDNS("8.8.8.8", "dns.google")`,
					expected:          types.Bool(true),
					description:       "should return true for valid FCrDNS with matching pattern",
				},
				{
					name:              "failure-with-pattern",
					addr:              "8.8.8.8",
					reverseMockReturn: []string{"dns.google."},
					forwardMockReturn: map[string][]string{"dns.google": {"8.8.8.8"}},
					expression:        `verifyFCrDNS("8.8.8.8", "wrong.pattern")`,
					expected:          types.Bool(false),
					description:       "should return false for FCrDNS with non-matching pattern",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					internal.DNSLookupAddr = func(addr string) ([]string, error) {
						if addr == tt.addr {
							return tt.reverseMockReturn, tt.reverseMockError
						}
						return nil, errors.New("unexpected address for reverse lookup")
					}
					internal.DNSLookupHost = func(host string) ([]string, error) {
						host = strings.TrimSuffix(host, ".")
						if ips, ok := tt.forwardMockReturn[host]; ok {
							return ips, nil
						}
						if err, ok := tt.forwardMockError[host]; ok {
							return nil, err
						}
						return nil, &net.DNSError{IsNotFound: true}
					}

					prog, err := Compile(env, tt.expression)
					if err != nil {
						t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
					}

					result, _, err := prog.Eval(map[string]interface{}{})
					if err != nil {
						t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
					}
					if result.Equal(tt.expected) != types.True {
						t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
					}
				})
			}
		})

		t.Run("arpaReverseIP", func(t *testing.T) {
			tests := []struct {
				name        string
				expression  string
				expected    types.String
				description string
				evalError   bool
			}{
				{
					name:        "ipv4",
					expression:  `arpaReverseIP("1.2.3.4")`,
					expected:    types.String("4.3.2.1"),
					description: "should correctly reverse an IPv4 address",
				},
				{
					name:        "ipv6",
					expression:  `arpaReverseIP("2001:db8::1")`,
					expected:    types.String("1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2"),
					description: "should correctly reverse an IPv6 address",
				},
				{
					name:        "ipv6-full",
					expression:  `arpaReverseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")`,
					expected:    types.String("4.3.3.7.0.7.3.0.e.2.a.8.0.0.0.0.0.0.0.0.3.a.5.8.8.b.d.0.1.0.0.2"),
					description: "should correctly reverse a fully expanded IPv6 address",
				},
				{
					name:        "ipv6-loopback",
					expression:  `arpaReverseIP("::1")`,
					expected:    types.String("1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0"),
					description: "should correctly reverse the IPv6 loopback address",
				},
				{
					name:        "invalid-ip",
					expression:  `arpaReverseIP("not-an-ip")`,
					evalError:   true,
					description: "should error on an invalid IP",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					prog, err := Compile(env, tt.expression)
					if err != nil {
						t.Fatalf("failed to compile expression %q: %v", tt.expression, err)
					}

					result, _, err := prog.Eval(map[string]interface{}{})
					if tt.evalError {
						if err == nil {
							t.Errorf("%s: expected an evaluation error, but got none", tt.description)
						}
						return
					}
					if err != nil {
						t.Fatalf("failed to evaluate expression %q: %v", tt.expression, err)
					}
					if result.Equal(tt.expected) != types.True {
						t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
					}
				})
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
