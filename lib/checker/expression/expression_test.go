package expression

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/TecharoHQ/anubis/internal/dns"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

// newTestDNS builds a DNS resolver backed by an in-memory store so tests never
// touch persistent storage. The resolver itself still performs real lookups
// against the network when its functions are evaluated.
func newTestDNS(t *testing.T) *dns.Dns {
	t.Helper()

	ctx := t.Context()
	memStore := memory.New(ctx)
	cache := dns.NewDNSCache(300, 300, memStore)
	return dns.New(ctx, cache)
}

// buildImpl compiles a single CEL expression into a checker, wiring an
// in-memory-backed DNS resolver onto the build context.
func buildImpl(t *testing.T, expr string) checker.Impl {
	t.Helper()

	ctx := dns.With(t.Context(), newTestDNS(t))
	impl, err := Factory{}.Create(ctx, jsonOf(t, Config{Expression: expr}))
	if err != nil {
		t.Fatalf("can't build impl for %q: %v", expr, err)
	}

	return impl
}

// skipWithoutNetwork skips a test when DONT_USE_NETWORK is set (e.g. Nix builds
// and other sandboxed environments without egress).
func skipWithoutNetwork(t *testing.T) {
	t.Helper()

	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
	}
}

func TestFactoryIsCheckerFactory(t *testing.T) {
	if _, ok := (any(Factory{})).(checker.Factory); !ok {
		t.Fatal("Factory is not an instance of checker.Factory")
	}
}

func TestImplIsCheckerImpl(t *testing.T) {
	if _, ok := (any(&Impl{})).(checker.Impl); !ok {
		t.Fatal("Impl is not an instance of checker.Impl")
	}
}

func TestFactoryValidateConfig(t *testing.T) {
	f := Factory{}

	for _, tt := range []struct {
		name string
		data []byte
		err  error
	}{
		{
			name: "valid expression",
			data: []byte(`"userAgent == \"Mozilla\""`),
		},
		{
			name: "valid all",
			data: []byte(`{"all": ["userAgent == \"Mozilla\"", "path == \"/\""]}`),
		},
		{
			name: "valid any",
			data: []byte(`{"any": ["userAgent == \"Mozilla\"", "path == \"/\""]}`),
		},
		{
			name: "not json",
			data: []byte(`]`),
			err:  config.ErrUnparseableConfig,
		},
		{
			name: "empty",
			data: []byte(`{}`),
			err:  config.ErrExpressionEmpty,
		},
		{
			name: "can't have both",
			data: []byte(`{"all": ["path == \"/\""], "any": ["path == \"/\""]}`),
			err:  config.ErrExpressionCantHaveBoth,
		},
		{
			name: "invalid cel",
			data: []byte(`"this is not @@@ valid cel"`),
			err:  ErrCantCompile,
		},
		{
			name: "references unknown variable",
			data: []byte(`"nonexistentVariable == \"x\""`),
			err:  ErrCantCompile,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			if err := f.ValidateConfig(t.Context(), data); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Fatal("validation didn't do what was expected")
			}
		})
	}
}

func jsonOf(t *testing.T, inp Config) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(&inp)
	if err != nil {
		t.Fatalf("can't marshal Config, this is a bug: %v", err)
	}

	return json.RawMessage(data)
}

func TestEndToEnd(t *testing.T) {
	f := Factory{}

	for _, tt := range []struct {
		name      string
		c         Config
		method    string
		path      string
		userAgent string
		match     bool
	}{
		{
			name:      "single expression match",
			c:         Config{Expression: `userAgent == "Mozilla"`},
			path:      "/",
			userAgent: "Mozilla",
			match:     true,
		},
		{
			name:      "single expression no match",
			c:         Config{Expression: `userAgent == "Mozilla"`},
			path:      "/",
			userAgent: "curl/8.0",
			match:     false,
		},
		{
			name:      "all both true",
			c:         Config{All: []string{`userAgent == "Mozilla"`, `path == "/admin"`}},
			path:      "/admin",
			userAgent: "Mozilla",
			match:     true,
		},
		{
			name:      "all one false",
			c:         Config{All: []string{`userAgent == "Mozilla"`, `path == "/admin"`}},
			path:      "/",
			userAgent: "Mozilla",
			match:     false,
		},
		{
			name:      "any one true",
			c:         Config{Any: []string{`userAgent == "Mozilla"`, `path == "/admin"`}},
			path:      "/admin",
			userAgent: "curl/8.0",
			match:     true,
		},
		{
			name:      "any none true",
			c:         Config{Any: []string{`userAgent == "Mozilla"`, `path == "/admin"`}},
			path:      "/",
			userAgent: "curl/8.0",
			match:     false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl, err := f.Create(t.Context(), jsonOf(t, tt.c))
			if err != nil {
				t.Fatalf("can't build impl: %v", err)
			}

			method := tt.method
			if method == "" {
				method = http.MethodGet
			}

			r, err := http.NewRequest(method, tt.path, nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}
			r.Header.Set("User-Agent", tt.userAgent)

			t.Log(impl.Hash())
			match, err := impl.Check(r)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if tt.match != match {
				t.Errorf("match: %v, wanted: %v", match, tt.match)
			}
		})
	}
}

func TestEndToEndSubrequest(t *testing.T) {
	f := Factory{}
	ctx := checker.WithSubrequestMode(t.Context(), true)

	impl, err := f.Create(ctx, jsonOf(t, Config{Expression: `path == "/admin"`}))
	if err != nil {
		t.Fatalf("can't build impl: %v", err)
	}

	for _, candidate := range []string{"X-Original-URI", "X-Forwarded-Uri"} {
		t.Run(candidate, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}
			r.Header.Set(candidate, "/admin")

			match, err := impl.Check(r)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if !match {
				t.Errorf("expected subrequest path %q to match, but it didn't", "/admin")
			}
		})
	}
}

// TestVariables exercises every variable exposed to CEL programs.
func TestVariables(t *testing.T) {
	for _, tt := range []struct {
		name  string
		url   string
		expr  string
		setup func(*http.Request)
		want  bool
	}{
		{
			name:  "remoteAddress",
			url:   "http://example.com/",
			expr:  `remoteAddress == "1.2.3.4"`,
			setup: func(r *http.Request) { r.Header.Set("X-Real-Ip", "1.2.3.4") },
			want:  true,
		},
		{
			name:  "contentLength",
			url:   "http://example.com/",
			expr:  `contentLength == 5`,
			setup: func(r *http.Request) { r.ContentLength = 5 },
			want:  true,
		},
		{
			name: "host",
			url:  "http://example.com/",
			expr: `host == "example.com"`,
			want: true,
		},
		{
			name: "method",
			url:  "http://example.com/",
			expr: `method == "GET"`,
			want: true,
		},
		{
			name:  "userAgent",
			url:   "http://example.com/",
			expr:  `userAgent == "TestAgent/1.0"`,
			setup: func(r *http.Request) { r.Header.Set("User-Agent", "TestAgent/1.0") },
			want:  true,
		},
		{
			name: "path",
			url:  "http://example.com/foo/bar",
			expr: `path == "/foo/bar"`,
			want: true,
		},
		{
			name: "query",
			url:  "http://example.com/?format=json&page=2",
			expr: `query["format"] == "json"`,
			want: true,
		},
		{
			name:  "headers",
			url:   "http://example.com/",
			expr:  `headers["X-Test"] == "yes"`,
			setup: func(r *http.Request) { r.Header.Set("X-Test", "yes") },
			want:  true,
		},
		{
			// Load averages are sampled from the host; on any machine actively
			// running this test the one-minute average is above zero.
			name: "load_1m",
			url:  "http://example.com/",
			expr: `load_1m > 0.0`,
			want: true,
		},
		{
			name: "load_5m",
			url:  "http://example.com/",
			expr: `load_5m > 0.0`,
			want: true,
		},
		{
			name: "load_15m",
			url:  "http://example.com/",
			expr: `load_15m > 0.0`,
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl := buildImpl(t, tt.expr)

			r, err := http.NewRequest(http.MethodGet, tt.url, nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}
			if tt.setup != nil {
				tt.setup(r)
			}

			match, err := impl.Check(r)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if match != tt.want {
				t.Errorf("expr %q: match = %v, want %v", tt.expr, match, tt.want)
			}
		})
	}
}

// TestFunctions exercises the CEL functions that do not require network access.
func TestFunctions(t *testing.T) {
	for _, tt := range []struct {
		name string
		expr string
		want bool
	}{
		{
			name: "missingHeader present",
			expr: `missingHeader(headers, "User-Agent")`,
			want: false,
		},
		{
			name: "missingHeader absent",
			expr: `missingHeader(headers, "X-Definitely-Absent")`,
			want: true,
		},
		{
			name: "regexSafe escapes metacharacters",
			expr: `regexSafe("1.1.1.1") == "1\\.1\\.1\\.1"`,
			want: true,
		},
		{
			name: "segments splits path",
			expr: `segments("/foo/bar/baz")[1] == "bar"`,
			want: true,
		},
		{
			name: "segments size",
			expr: `size(segments("/foo/bar/baz")) == 3`,
			want: true,
		},
		{
			name: "arpaReverseIP ipv4",
			expr: `arpaReverseIP("1.2.3.4") == "4.3.2.1"`,
			want: true,
		},
		{
			name: "arpaReverseIP ipv6",
			expr: `arpaReverseIP("2001:db8::1").endsWith("8.b.d.0.1.0.0.2")`,
			want: true,
		},
		{
			name: "randInt within bound",
			expr: `randInt(5) >= 0 && randInt(5) < 5`,
			want: true,
		},
		{
			name: "randInt of one is zero",
			expr: `randInt(1) == 0`,
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl := buildImpl(t, tt.expr)

			r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}
			r.Header.Set("User-Agent", "TestAgent/1.0")

			match, err := impl.Check(r)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if match != tt.want {
				t.Errorf("expr %q: match = %v, want %v", tt.expr, match, tt.want)
			}
		})
	}
}

// TestDNSFunctions exercises the CEL functions that perform real DNS lookups.
// They are skipped when DONT_USE_NETWORK is set.
func TestDNSFunctions(t *testing.T) {
	skipWithoutNetwork(t)

	for _, tt := range []struct {
		name string
		expr string
		want bool
	}{
		{
			// techaro.lol resolves to at least one address.
			name: "lookupHost resolves",
			expr: `size(lookupHost("techaro.lol")) > 0`,
			want: true,
		},
		{
			// 1.1.1.1 has a stable PTR record.
			name: "reverseDNS resolves",
			expr: `"one.one.one.one" in reverseDNS("1.1.1.1")`,
			want: true,
		},
		{
			name: "verifyFCrDNS valid",
			expr: `verifyFCrDNS("1.1.1.1")`,
			want: true,
		},
		{
			name: "verifyFCrDNS with matching pattern",
			expr: `verifyFCrDNS("1.1.1.1", "one.one.one.one")`,
			want: true,
		},
		{
			name: "verifyFCrDNS with non-matching pattern",
			expr: `verifyFCrDNS("1.1.1.1", "not.cloudflare.example")`,
			want: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl := buildImpl(t, tt.expr)

			r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}

			match, err := impl.Check(r)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if match != tt.want {
				t.Errorf("expr %q: match = %v, want %v", tt.expr, match, tt.want)
			}
		})
	}
}
