package lib

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
)

// passChallengeCookie submits a solved zero difficulty challenge, optionally
// without the verification cookie as if the browser had dropped it.
func passChallengeCookie(t *testing.T, srv *Server, ts *httptest.Server, c challengeResp, userAgent string, withCookie bool) *http.Response {
	t.Helper()

	q := url.Values{}
	q.Set("id", c.ID)
	q.Set("response", internal.SHA256sum(c.Challenge+"0"))
	q.Set("nonce", "0")
	q.Set("redir", "/")
	q.Set("elapsedTime", "420")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/.within.website/x/cmd/anubis/api/pass-challenge?"+q.Encode(), nil)
	if err != nil {
		t.Fatalf("can't make request: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: srv.cookieName(anubis.TestCookieName), Value: c.ID})
	}

	cli := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("can't do request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func TestPassChallengeMissingVerificationCookie(t *testing.T) {
	type step struct {
		reuse      bool   // reuse the previous step's challenge instead of issuing a new one
		id         string // override the challenge ID
		userAgent  string // defaults to Mozilla/5.0
		withCookie bool
		wantStatus int
		wantAuth   bool
	}

	for _, tt := range []struct {
		name  string
		steps []step
	}{
		{
			name: "cookie present passes",
			steps: []step{
				{withCookie: true, wantStatus: http.StatusFound, wantAuth: true},
			},
		},
		{
			name: "first miss retries then passes",
			steps: []step{
				{wantStatus: http.StatusFound},
				{withCookie: true, wantStatus: http.StatusFound, wantAuth: true},
			},
		},
		{
			name: "repeated misses end in an error",
			steps: []step{
				{wantStatus: http.StatusFound},
				{wantStatus: http.StatusInternalServerError},
				{wantStatus: http.StatusInternalServerError},
			},
		},
		{
			name: "retry is per client",
			steps: []step{
				{wantStatus: http.StatusFound},
				{userAgent: "Mozilla/5.0 other", wantStatus: http.StatusFound},
				{wantStatus: http.StatusInternalServerError},
			},
		},
		{
			name: "unknown challenge is not retried",
			steps: []step{
				{id: "not-a-challenge", wantStatus: http.StatusInternalServerError},
			},
		},
		{
			name: "spent challenge is not retried",
			steps: []step{
				{withCookie: true, wantStatus: http.StatusFound, wantAuth: true},
				{reuse: true, wantStatus: http.StatusInternalServerError},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
			ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
			t.Cleanup(ts.Close)

			var c challengeResp
			for i, st := range tt.steps {
				if !st.reuse {
					c = makeChallenge(t, ts, httpClient(t))
				}
				if st.id != "" {
					c.ID = st.id
				}
				if st.userAgent == "" {
					st.userAgent = "Mozilla/5.0"
				}

				resp := passChallengeCookie(t, srv, ts, c, st.userAgent, st.withCookie)

				if resp.StatusCode != st.wantStatus {
					t.Fatalf("step %d: wanted status %d, got %d", i, st.wantStatus, resp.StatusCode)
				}
				if got := authCookie(srv, resp) != nil; got != st.wantAuth {
					t.Fatalf("step %d: wanted auth cookie %v, got %v", i, st.wantAuth, got)
				}
				if st.wantStatus == http.StatusFound && !st.wantAuth {
					if loc := resp.Header.Get("Location"); loc != "/" {
						t.Fatalf("step %d: retry should redirect to the original destination, got %q", i, loc)
					}
				}
			}
		})
	}
}

// TestMissingVerificationCookieRetryIssuesFreshChallenge follows the retry
// redirect and makes sure the client gets a new challenge and verification
// cookie rather than an error page.
func TestMissingVerificationCookieRetryIssuesFreshChallenge(t *testing.T) {
	srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	t.Cleanup(ts.Close)

	cli := httpClient(t)
	c := makeChallenge(t, ts, cli)

	resp := passChallengeCookie(t, srv, ts, c, "Mozilla/5.0", false)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("wanted retry redirect, got status %d", resp.StatusCode)
	}

	loc, err := resp.Location()
	if err != nil {
		t.Fatalf("retry redirect has no location: %v", err)
	}

	next, err := cli.Get(loc.String())
	if err != nil {
		t.Fatalf("can't follow retry redirect: %v", err)
	}
	t.Cleanup(func() { _ = next.Body.Close() })

	var fresh *http.Cookie
	for _, ckie := range next.Cookies() {
		if ckie.Name == srv.cookieName(anubis.TestCookieName) && ckie.Value != "" {
			fresh = ckie
		}
	}
	if fresh == nil {
		t.Fatal("retry did not issue a new verification cookie")
	}
	if fresh.Value == c.ID {
		t.Error("retry reused the old challenge")
	}
}
