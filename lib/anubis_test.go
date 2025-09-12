package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/lib/thoth/thothmock"
)

// TLogWriter implements io.Writer by logging each line to t.Log.
type TLogWriter struct {
	t *testing.T
}

// NewTLogWriter returns an io.Writer that sends output to t.Log.
func NewTLogWriter(t *testing.T) io.Writer {
	return &TLogWriter{t: t}
}

// Write splits input on newlines and logs each line separately.
func (w *TLogWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		if line != "" {
			w.t.Log(line)
		}
	}
	return len(p), nil
}

func loadPolicies(t *testing.T, fname string, difficulty int) *policy.ParsedConfig {
	t.Helper()

	ctx := thothmock.WithMockThoth(t)

	if fname == "" {
		fname = "./testdata/test_config.yaml"
	}

	t.Logf("loading policy file: %s", fname)

	anubisPolicy, err := LoadPoliciesOrDefault(ctx, fname, difficulty)
	if err != nil {
		t.Fatal(err)
	}

	return anubisPolicy
}

func spawnAnubis(t *testing.T, opts Options) *Server {
	t.Helper()

	if opts.Policy == nil {
		opts.Policy = loadPolicies(t, "", 4)
	}

	s, err := New(opts)
	if err != nil {
		t.Fatalf("can't construct libanubis.Server: %v", err)
	}

	s.logger = slog.New(slog.NewJSONHandler(&TLogWriter{t: t}, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	return s
}

type challengeResp struct {
	ID        string `json:"id"`
	Challenge string `json:"challenge"`
}

func makeChallenge(t *testing.T, ts *httptest.Server, cli *http.Client) challengeResp {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/.within.website/x/cmd/anubis/api/make-challenge", nil)
	if err != nil {
		t.Fatalf("can't make request: %v", err)
	}

	q := req.URL.Query()
	q.Set("redir", "/")
	req.URL.RawQuery = q.Encode()

	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("can't request challenge: %v", err)
	}
	defer resp.Body.Close()

	var chall challengeResp
	if err := json.NewDecoder(resp.Body).Decode(&chall); err != nil {
		t.Fatalf("can't read challenge response body: %v", err)
	}

	return chall
}

func handleChallengeZeroDifficulty(t *testing.T, ts *httptest.Server, cli *http.Client, chall challengeResp) *http.Response {
	t.Helper()

	t.Logf("%#v", chall)

	nonce := 0
	elapsedTime := 420
	redir := "/"
	calculated := ""
	calcString := fmt.Sprintf("%s%d", chall.Challenge, nonce)
	calculated = internal.SHA256sum(calcString)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/.within.website/x/cmd/anubis/api/pass-challenge", nil)
	if err != nil {
		t.Fatalf("can't make request: %v", err)
	}

	q := req.URL.Query()
	q.Set("response", calculated)
	q.Set("nonce", fmt.Sprint(nonce))
	q.Set("redir", redir)
	q.Set("elapsedTime", fmt.Sprint(elapsedTime))
	q.Set("id", chall.ID)
	req.URL.RawQuery = q.Encode()

	t.Log(q.Encode())

	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("can't do request: %v", err)
	}

	return resp
}

type loggingCookieJar struct {
	t       *testing.T
	lock    sync.Mutex
	cookies map[string][]*http.Cookie
}

func (lcj *loggingCookieJar) Cookies(u *url.URL) []*http.Cookie {
	lcj.lock.Lock()
	defer lcj.lock.Unlock()

	// XXX(Xe): This is not RFC compliant in the slightest.
	result, ok := lcj.cookies[u.Host]
	if !ok {
		return nil
	}

	lcj.t.Logf("requested cookies for %s", u)

	for _, ckie := range result {
		lcj.t.Logf("get cookie: <- %s", ckie)
	}

	return result
}

func (lcj *loggingCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	lcj.lock.Lock()
	defer lcj.lock.Unlock()

	for _, ckie := range cookies {
		lcj.t.Logf("set cookie: %s -> %s", u, ckie)
	}

	// XXX(Xe): This is not RFC compliant in the slightest.
	lcj.cookies[u.Host] = append(lcj.cookies[u.Host], cookies...)
}

type userAgentRoundTripper struct {
	rt http.RoundTripper
}

func (u *userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only set if not already present
	req = req.Clone(req.Context()) // avoid mutating original request
	req.Header.Set("User-Agent", "Mozilla/5.0")
	return u.rt.RoundTrip(req)
}

func httpClient(t *testing.T) *http.Client {
	t.Helper()

	cli := &http.Client{
		Jar: &loggingCookieJar{t: t, cookies: map[string][]*http.Cookie{}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &userAgentRoundTripper{
			rt: http.DefaultTransport,
		},
	}

	return cli
}

func TestLoadPolicies(t *testing.T) {
	for _, fname := range []string{"botPolicies.yaml"} {
		t.Run(fname, func(t *testing.T) {
			fin, err := data.BotPolicies.Open(fname)
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := policy.ParseConfig(t.Context(), fin, fname, 4); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Regression test for CVE-2025-24369
func TestCVE2025_24369(t *testing.T) {
	pol := loadPolicies(t, "", anubis.DefaultDifficulty)

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)
	chall := makeChallenge(t, ts, cli)
	resp := handleChallengeZeroDifficulty(t, ts, cli, chall)

	if resp.StatusCode == http.StatusFound {
		t.Log("Regression on CVE-2025-24369")
		t.Errorf("wanted HTTP status %d, got: %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestCookieCustomExpiration(t *testing.T) {
	pol := loadPolicies(t, "testdata/zero_difficulty.yaml", 0)
	ckieExpiration := 10 * time.Minute

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,

		CookieExpiration: ckieExpiration,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)
	chall := makeChallenge(t, ts, cli)

	resp := handleChallengeZeroDifficulty(t, ts, cli, chall)

	if resp.StatusCode != http.StatusFound {
		resp.Write(os.Stderr)
		t.Errorf("wanted %d, got: %d", http.StatusFound, resp.StatusCode)
	}

	var ckie *http.Cookie
	for _, cookie := range resp.Cookies() {
		t.Logf("%#v", cookie)
		if cookie.Name == anubis.CookieName {
			ckie = cookie
			break
		}
	}
	if ckie == nil {
		t.Errorf("Cookie %q not found", anubis.CookieName)
		return
	}
}

func TestCookieSettings(t *testing.T) {
	pol := loadPolicies(t, "testdata/zero_difficulty.yaml", 0)

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,

		CookieDomain:      "127.0.0.1",
		CookiePartitioned: true,
		CookieSecure:      true,
		CookieExpiration:  anubis.CookieDefaultExpirationTime,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)
	chall := makeChallenge(t, ts, cli)

	resp := handleChallengeZeroDifficulty(t, ts, cli, chall)

	if resp.StatusCode != http.StatusFound {
		resp.Write(os.Stderr)
		t.Errorf("wanted %d, got: %d", http.StatusFound, resp.StatusCode)
	}

	var ckie *http.Cookie
	for _, cookie := range resp.Cookies() {
		t.Logf("%#v", cookie)
		if cookie.Name == anubis.CookieName {
			ckie = cookie
			break
		}
	}
	if ckie == nil {
		t.Errorf("Cookie %q not found", anubis.CookieName)
		return
	}

	if ckie.Domain != "127.0.0.1" {
		t.Errorf("cookie domain is wrong, wanted 127.0.0.1, got: %s", ckie.Domain)
	}

	if ckie.Partitioned != srv.opts.CookiePartitioned {
		t.Errorf("wanted partitioned flag %v, got: %v", srv.opts.CookiePartitioned, ckie.Partitioned)
	}

	if ckie.Secure != srv.opts.CookieSecure {
		t.Errorf("wanted secure flag %v, got: %v", srv.opts.CookieSecure, ckie.Secure)
	}
}

func TestCheckDefaultDifficultyMatchesPolicy(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	for i := 1; i < 10; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			anubisPolicy := loadPolicies(t, "testdata/test_config_no_thresholds.yaml", i)

			s, err := New(Options{
				Next:           h,
				Policy:         anubisPolicy,
				ServeRobotsTXT: true,
			})
			if err != nil {
				t.Fatalf("can't construct libanubis.Server: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Add("X-Real-Ip", "127.0.0.1")

			cr, bot, err := s.check(req, s.logger)
			if err != nil {
				t.Fatal(err)
			}

			t.Log(cr.Name)

			if bot.Challenge.Difficulty != i {
				t.Errorf("Challenge.Difficulty is wrong, wanted %d, got: %d", i, bot.Challenge.Difficulty)
			}

			if bot.Challenge.ReportAs != i {
				t.Errorf("Challenge.ReportAs is wrong, wanted %d, got: %d", i, bot.Challenge.ReportAs)
			}
		})
	}
}

func TestBasePrefix(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	testCases := []struct {
		name       string
		basePrefix string
		path       string
		expected   string
	}{
		{
			name:       "no prefix",
			basePrefix: "/",
			path:       "/.within.website/x/cmd/anubis/api/make-challenge",
			expected:   "/.within.website/x/cmd/anubis/api/make-challenge",
		},
		{
			name:       "with prefix",
			basePrefix: "/myapp",
			path:       "/myapp/.within.website/x/cmd/anubis/api/make-challenge",
			expected:   "/myapp/.within.website/x/cmd/anubis/api/make-challenge",
		},
		{
			name:       "with prefix and trailing slash",
			basePrefix: "/myapp/",
			path:       "/myapp/.within.website/x/cmd/anubis/api/make-challenge",
			expected:   "/myapp/.within.website/x/cmd/anubis/api/make-challenge",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the global BasePrefix before each test
			anubis.BasePrefix = ""

			pol := loadPolicies(t, "", 4)

			srv := spawnAnubis(t, Options{
				Next:       h,
				Policy:     pol,
				BasePrefix: tc.basePrefix,
			})

			ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
			defer ts.Close()

			cli := httpClient(t)

			req, err := http.NewRequest(http.MethodPost, ts.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			q := req.URL.Query()
			q.Set("redir", tc.basePrefix)
			req.URL.RawQuery = q.Encode()

			// Test API endpoint with prefix
			resp, err := cli.Do(req)
			if err != nil {
				t.Fatalf("can't request challenge: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code %d, got: %d", http.StatusOK, resp.StatusCode)
			}

			var chall challengeResp
			if err := json.NewDecoder(resp.Body).Decode(&chall); err != nil {
				t.Fatalf("can't read challenge response body: %v", err)
			}

			if chall.Challenge == "" {
				t.Errorf("expected non-empty challenge")
			}

			// Test cookie path when passing challenge
			// Find a nonce that produces a hash with the required number of leading zeros
			nonce := 0
			var calculated string
			for {
				calcString := fmt.Sprintf("%s%d", chall.Challenge, nonce)
				calculated = internal.SHA256sum(calcString)
				if strings.HasPrefix(calculated, strings.Repeat("0", pol.DefaultDifficulty)) {
					break
				}
				nonce++
			}
			elapsedTime := 420
			redir := "/"

			cli.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}

			// Construct the correct path for pass-challenge
			passChallengePath := tc.path
			passChallengePath = passChallengePath[:strings.LastIndex(passChallengePath, "/")+1] + "pass-challenge"

			req, err = http.NewRequest(http.MethodGet, ts.URL+passChallengePath, nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			for _, ckie := range resp.Cookies() {
				req.AddCookie(ckie)
			}

			q = req.URL.Query()
			q.Set("response", calculated)
			q.Set("nonce", fmt.Sprint(nonce))
			q.Set("redir", redir)
			q.Set("elapsedTime", fmt.Sprint(elapsedTime))
			q.Set("id", chall.ID)
			req.URL.RawQuery = q.Encode()

			t.Log(req.URL.String())

			resp, err = cli.Do(req)
			if err != nil {
				t.Fatalf("can't do challenge passing: %v", err)
			}

			if resp.StatusCode != http.StatusFound {
				t.Errorf("wanted %d, got: %d", http.StatusFound, resp.StatusCode)
			}

			// Check cookie path
			var ckie *http.Cookie
			for _, cookie := range resp.Cookies() {
				if cookie.Name == anubis.CookieName {
					ckie = cookie
					break
				}
			}
			if ckie == nil {
				t.Errorf("Cookie %q not found", anubis.CookieName)
				return
			}

			expectedPath := "/"
			if tc.basePrefix != "" {
				expectedPath = strings.TrimSuffix(tc.basePrefix, "/") + "/"
			}

			if ckie.Path != expectedPath {
				t.Errorf("cookie path is wrong, wanted %s, got: %s", expectedPath, ckie.Path)
			}
		})
	}
}

func TestCustomStatusCodes(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.UserAgent())
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	statusMap := map[string]int{
		"ALLOW":     200,
		"CHALLENGE": 401,
		"DENY":      403,
	}

	pol := loadPolicies(t, "./testdata/aggressive_403.yaml", 4)

	srv := spawnAnubis(t, Options{
		Next:   h,
		Policy: pol,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	for userAgent, statusCode := range statusMap {
		t.Run(userAgent, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set("User-Agent", userAgent)

			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != statusCode {
				t.Errorf("wanted status code %d but got: %d", statusCode, resp.StatusCode)
			}
		})
	}
}

func TestCloudflareWorkersRule(t *testing.T) {
	for _, variant := range []string{"cel", "header"} {
		t.Run(variant, func(t *testing.T) {
			pol := loadPolicies(t, "./testdata/cloudflare-workers-"+variant+".yaml", 0)

			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "OK")
			})

			s, err := New(Options{
				Next:           h,
				Policy:         pol,
				ServeRobotsTXT: true,
			})
			if err != nil {
				t.Fatalf("can't construct libanubis.Server: %v", err)
			}

			t.Run("with-cf-worker-header", func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, "/", nil)
				if err != nil {
					t.Fatal(err)
				}

				req.Header.Add("X-Real-Ip", "127.0.0.1")
				req.Header.Add("Cf-Worker", "true")

				cr, _, err := s.check(req, s.logger)
				if err != nil {
					t.Fatal(err)
				}

				if cr.Rule != config.RuleDeny {
					t.Errorf("rule is wrong, wanted %s, got: %s", config.RuleDeny, cr.Rule)
				}
			})

			t.Run("no-cf-worker-header", func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, "/", nil)
				if err != nil {
					t.Fatal(err)
				}

				req.Header.Add("X-Real-Ip", "127.0.0.1")

				cr, _, err := s.check(req, s.logger)
				if err != nil {
					t.Fatal(err)
				}

				if cr.Rule != config.RuleAllow {
					t.Errorf("rule is wrong, wanted %s, got: %s", config.RuleAllow, cr.Rule)
				}
			})
		})
	}
}

func TestRuleChange(t *testing.T) {
	pol := loadPolicies(t, "testdata/rule_change.yaml", 0)
	ckieExpiration := 10 * time.Minute

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,

		CookieDomain:     "127.0.0.1",
		CookieExpiration: ckieExpiration,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)

	chall := makeChallenge(t, ts, cli)
	resp := handleChallengeZeroDifficulty(t, ts, cli, chall)

	if resp.StatusCode != http.StatusFound {
		resp.Write(os.Stderr)
		t.Errorf("wanted %d, got: %d", http.StatusFound, resp.StatusCode)
	}
}

func TestDenyAndRerouteAction(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.UserAgent())
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	pol := loadPolicies(t, "./testdata/deny_and_reroute_test.yaml", 4)

	srv := spawnAnubis(t, Options{
		Next:   h,
		Policy: pol,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	testCases := []struct {
		userAgent    string
		expectedCode int
		expectedURL  string
	}{
		{
			userAgent:    "REROUTE_ME",
			expectedCode: http.StatusTemporaryRedirect,
			expectedURL:  "https://example.com/tarpit",
		},
		{
			userAgent:    "DENY_ME",
			expectedCode: http.StatusOK, // From status_codes config
		},
		{
			userAgent:    "ALLOW_ME",
			expectedCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.userAgent, func(t *testing.T) {
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					// Don't follow redirects, we want to test the redirect response
					return http.ErrUseLastResponse
				},
			}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Set("User-Agent", tc.userAgent)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedCode {
				t.Errorf("wanted status code %d but got: %d", tc.expectedCode, resp.StatusCode)
			}

			if tc.expectedURL != "" {
				location := resp.Header.Get("Location")
				if location != tc.expectedURL {
					t.Errorf("wanted Location header %q but got: %q", tc.expectedURL, location)
				}
			}
		})
	}
}

func TestStripBasePrefixFromRequest(t *testing.T) {
	testCases := []struct {
		name            string
		basePrefix      string
		stripBasePrefix bool
		requestPath     string
		expectedPath    string
	}{
		{
			name:            "strip disabled - no change",
			basePrefix:      "/foo",
			stripBasePrefix: false,
			requestPath:     "/foo/bar",
			expectedPath:    "/foo/bar",
		},
		{
			name:            "strip enabled - removes prefix",
			basePrefix:      "/foo",
			stripBasePrefix: true,
			requestPath:     "/foo/bar",
			expectedPath:    "/bar",
		},
		{
			name:            "strip enabled - root becomes slash",
			basePrefix:      "/foo",
			stripBasePrefix: true,
			requestPath:     "/foo",
			expectedPath:    "/",
		},
		{
			name:            "strip enabled - trailing slash on base prefix",
			basePrefix:      "/foo/",
			stripBasePrefix: true,
			requestPath:     "/foo/bar",
			expectedPath:    "/bar",
		},
		{
			name:            "strip enabled - no prefix match",
			basePrefix:      "/foo",
			stripBasePrefix: true,
			requestPath:     "/other/bar",
			expectedPath:    "/other/bar",
		},
		{
			name:            "strip enabled - empty base prefix",
			basePrefix:      "",
			stripBasePrefix: true,
			requestPath:     "/foo/bar",
			expectedPath:    "/foo/bar",
		},
		{
			name:            "strip enabled - nested path",
			basePrefix:      "/app",
			stripBasePrefix: true,
			requestPath:     "/app/api/v1/users",
			expectedPath:    "/api/v1/users",
		},
		{
			name:            "strip enabled - exact match becomes root",
			basePrefix:      "/myapp",
			stripBasePrefix: true,
			requestPath:     "/myapp/",
			expectedPath:    "/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &Server{
				opts: Options{
					BasePrefix:      tc.basePrefix,
					StripBasePrefix: tc.stripBasePrefix,
				},
			}

			req := httptest.NewRequest(http.MethodGet, tc.requestPath, nil)
			originalPath := req.URL.Path

			result := srv.stripBasePrefixFromRequest(req)

			if result.URL.Path != tc.expectedPath {
				t.Errorf("expected path %q, got %q", tc.expectedPath, result.URL.Path)
			}

			// Ensure original request is not modified when no stripping should occur
			if !tc.stripBasePrefix || tc.basePrefix == "" || !strings.HasPrefix(tc.requestPath, strings.TrimSuffix(tc.basePrefix, "/")) {
				if result != req {
					t.Error("expected same request object when no modification needed")
				}
			} else {
				// Ensure original request is not modified when stripping occurs
				if req.URL.Path != originalPath {
					t.Error("original request was modified")
				}
			}
		})
	}
}

// TestChallengeFor_ErrNotFound makes sure that users with invalid challenge IDs
// in the test cookie don't get rejected by the database lookup failing.
func TestChallengeFor_ErrNotFound(t *testing.T) {
	pol := loadPolicies(t, "testdata/aggressive_403.yaml", 0)
	ckieExpiration := 10 * time.Minute
	const wrongCookie = "wrong cookie"

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,

		CookieDomain:     "127.0.0.1",
		CookieExpiration: ckieExpiration,
	})

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")
	req.Header.Set("User-Agent", "CHALLENGE")
	req.AddCookie(&http.Cookie{Name: anubis.TestCookieName, Value: wrongCookie})

	w := httptest.NewRecorder()
	srv.maybeReverseProxyOrPage(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	body := new(strings.Builder)
	_, err := io.Copy(body, resp.Body)
	if err != nil {
		t.Fatalf("reading body should not fail: %v", err)
	}

	t.Run("make sure challenge page is issued", func(t *testing.T) {
		if !strings.Contains(body.String(), "anubis_challenge") {
			t.Error("should get a challenge page")
		}

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("should get a 401 Unauthorized, got: %d", resp.StatusCode)
		}
	})

	t.Run("make sure that the body is not an error page", func(t *testing.T) {
		if strings.Contains(body.String(), "reject.webp") {
			t.Error("should not get an internal server error")
		}
	})

	t.Run("make sure new test cookie is issued", func(t *testing.T) {
		found := false
		for _, cookie := range resp.Cookies() {
			if cookie.Name == anubis.TestCookieName {
				if cookie.Value == wrongCookie {
					t.Error("a new challenge cookie should be issued")
				}
				found = true
			}
		}
		if !found {
			t.Error("a new test cookie should be set")
		}
	})
}

func TestPassChallengeXSS(t *testing.T) {
	pol := loadPolicies(t, "", anubis.DefaultDifficulty)

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: pol,
	})

	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)
	chall := makeChallenge(t, ts, cli)

	testCases := []struct {
		name  string
		redir string
	}{
		{
			name:  "javascript alert",
			redir: "javascript:alert('xss')",
		},
		{
			name:  "vbscript",
			redir: "vbscript:msgbox(\"XSS\")",
		},
		{
			name:  "data url",
			redir: "data:text/html;base64,PHNjcmlwdD5hbGVydCgneHNzJyk8L3NjcmlwdD4=",
		},
	}

	t.Run("with test cookie", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				nonce := 0
				elapsedTime := 420
				calculated := ""
				calcString := fmt.Sprintf("%s%d", chall.Challenge, nonce)
				calculated = internal.SHA256sum(calcString)

				req, err := http.NewRequest(http.MethodGet, ts.URL+"/.within.website/x/cmd/anubis/api/pass-challenge", nil)
				if err != nil {
					t.Fatalf("can't make request: %v", err)
				}

				q := req.URL.Query()
				q.Set("response", calculated)
				q.Set("nonce", fmt.Sprint(nonce))
				q.Set("redir", tc.redir)
				q.Set("elapsedTime", fmt.Sprint(elapsedTime))
				req.URL.RawQuery = q.Encode()

				u, err := url.Parse(ts.URL)
				if err != nil {
					t.Fatal(err)
				}

				for _, ckie := range cli.Jar.Cookies(u) {
					if ckie.Name == anubis.TestCookieName {
						req.AddCookie(ckie)
					}
				}

				resp, err := cli.Do(req)
				if err != nil {
					t.Fatalf("can't do request: %v", err)
				}

				body, _ := io.ReadAll(resp.Body)

				if bytes.Contains(body, []byte(tc.redir)) {
					t.Log(string(body))
					t.Error("found XSS in HTML body")
				}

				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("wanted status %d, got %d. body: %s", http.StatusBadRequest, resp.StatusCode, body)
				}
			})
		}
	})

	t.Run("no test cookie", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				nonce := 0
				elapsedTime := 420
				calculated := ""
				calcString := fmt.Sprintf("%s%d", chall.Challenge, nonce)
				calculated = internal.SHA256sum(calcString)

				req, err := http.NewRequest(http.MethodGet, ts.URL+"/.within.website/x/cmd/anubis/api/pass-challenge", nil)
				if err != nil {
					t.Fatalf("can't make request: %v", err)
				}

				q := req.URL.Query()
				q.Set("response", calculated)
				q.Set("nonce", fmt.Sprint(nonce))
				q.Set("redir", tc.redir)
				q.Set("elapsedTime", fmt.Sprint(elapsedTime))
				req.URL.RawQuery = q.Encode()

				resp, err := cli.Do(req)
				if err != nil {
					t.Fatalf("can't do request: %v", err)
				}

				body, _ := io.ReadAll(resp.Body)

				if bytes.Contains(body, []byte(tc.redir)) {
					t.Log(string(body))
					t.Error("found XSS in HTML body")
				}

				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("wanted status %d, got %d. body: %s", http.StatusBadRequest, resp.StatusCode, body)
				}
			})
		}
	})
}

func TestXForwardedForNoDoubleComma(t *testing.T) {
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Forwarded-For", r.Header.Get("X-Forwarded-For"))
		fmt.Fprintln(w, "OK")
	})

	h = internal.XForwardedForToXRealIP(h)
	h = internal.XForwardedForUpdate(false, h)

	pol := loadPolicies(t, "testdata/permissive.yaml", 4)

	srv := spawnAnubis(t, Options{
		Next:   h,
		Policy: pol,
	})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("X-Real-Ip", "10.0.0.1")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("response status is wrong, wanted %d but got: %s", http.StatusOK, resp.Status)
	}

	if xff := resp.Header.Get("X-Forwarded-For"); strings.HasPrefix(xff, ",,") {
		t.Errorf("X-Forwarded-For has two leading commas: %q", xff)
	}
}
