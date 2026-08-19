package test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/mxschmitt/playwright-go"
)

// blobBlockingCSP is the Content-Security-Policy (CSP) reported in
// TecharoHQ/anubis#1864. This CSP doesn't set worker-src, so worker-src
// falls back through script-src to `default-src 'self'`, which does not
// cover blob: URIs.
//
// In order to reduce server request pressure, Anubis pre-fetches browser
// worker JS from the server and packs it into a blob: URI. This policy
// makes browsers reject that behaviour as an error event, leading the
// workers to all die off and then users to be SOL.
//
// This policy also enforces Anubis to fall back to the old behaviour
// where each worker fetches its own copy of the worker script in one
// request per thread. This kinda sucks, but such is life in late-stage
// capitalism.
const blobBlockingCSP = "default-src 'self' 'unsafe-eval' 'unsafe-inline'; img-src * data:;"

func TestPlaywrightBlobWorkerBlockedByCSP(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("SKIP_INTEGRATION was set")
		return
	}

	startPlaywright(t)

	pw := setupPlaywright(t)
	anubisURL := spawnAnubisWithCSP(t, "", blobBlockingCSP)

	for _, typ := range []playwright.BrowserType{pw.Chromium, pw.Firefox, pw.WebKit} {
		t.Run(typ.Name(), func(t *testing.T) {
			browser, err := typ.Connect(buildBrowserConnect(typ.Name()))
			if err != nil {
				t.Fatalf("could not connect to remote browser: %v", err)
			}
			defer browser.Close()

			ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
				AcceptDownloads: playwright.Bool(false),
				ExtraHttpHeaders: map[string]string{
					"X-Real-Ip": placeholderIP,
				},
				UserAgent: playwright.String("Mozilla/5.0 (X11; Linux x86_64; rv:136.0) Gecko/20100101 Firefox/136.0"),
			})
			if err != nil {
				t.Fatalf("could not create context: %v", err)
			}
			defer ctx.Close()

			page, err := ctx.NewPage()
			if err != nil {
				t.Fatalf("could not create page: %v", err)
			}
			defer page.Close()

			page.OnConsole(func(msg playwright.ConsoleMessage) {
				t.Logf("console: %s", msg.Text())
			})

			if _, err := page.Goto(anubisURL, playwright.PageGotoOptions{
				Timeout: playwright.Float(float64(playwrightMaxTime.Milliseconds())),
			}); err != nil {
				t.Fatalf("could not navigate to test server: %v", err)
			}

			if err := page.Locator("#anubis-test").WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(float64(challengeSolveTimeout.Milliseconds())),
			}); err != nil {
				t.Fatal(pwFail(t, page, "challenge did not complete under a CSP that forbids blob: workers: %v", err))
			}
		})
	}
}

// challengeSolveTimeout is the maximum amount of time browsers get to solve
// test challenges. By default this _should_ be good enough on the hardware
// that CI runs on. If this ends up being a bad assumption, revise this
// constant and document the tale of woe here.
const challengeSolveTimeout = 30 * time.Second

// spawnAnubisWithCSP starts Anubis with an additional inline middleware
// that stamps every request with the given Content-Security-Policy, which
// imitates the behaviour in TecharoHQ/anubis#1864 that caused workers to
// be unable to spawn.
//
// If you set no csp value, no Content-Security-Policy is injected.
func spawnAnubisWithCSP(t *testing.T, basePrefix, csp string) string {
	t.Helper()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><span id=anubis-test>%d</span></body></html>", time.Now().Unix())
	})

	policy, err := libanubis.LoadPoliciesOrDefault(t.Context(), "", anubis.DefaultDifficulty, "info", false)
	if err != nil {
		t.Fatal(err)
	}

	// Bind loopback explicitly: binding every interface makes ts.URL the
	// unspecified address (http://[::]:port), which Firefox refuses to navigate
	// to with NS_ERROR_CONNECTION_REFUSED.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("can't listen on random port: %v", err)
	}

	// XXX(Xe): I'd love to use httptest.NewServer for this, but we need to know
	// the address of the target ahead of time.
	addr := listener.Addr().(*net.TCPAddr)
	host := "localhost"
	port := strconv.Itoa(addr.Port)

	s, err := libanubis.New(libanubis.Options{
		Next:             h,
		Policy:           policy,
		ServeRobotsTXT:   true,
		Target:           "http://" + host + ":" + port,
		BasePrefix:       basePrefix,
		CookieExpiration: anubis.CookieDefaultExpirationTime,
	})
	if err != nil {
		t.Fatalf("can't construct libanubis.Server: %v", err)
	}

	var handler http.Handler = s
	if csp != "" {
		inner := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", csp)
			inner.ServeHTTP(w, r)
		})
	}

	ts := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	ts.Start()
	t.Log(ts.URL)

	t.Cleanup(func() {
		ts.Close()
	})

	return ts.URL
}
