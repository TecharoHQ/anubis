//go:build integration

// Integration tests for Anubis, using Playwright.
//
// These tests require an already running Anubis and Playwright server.
//
// Anubis must be configured to redirect to the server started by the test suite.
// The bind address and the Anubis server can be specified using the flags `-bind` and `-anubis` respectively.
//
// Playwright must be started in server mode using `npx playwright@1.50.1 run-server --port 3000`.
// The version must match the minor used by the playwright-go package.
//
// On unsupported systems you may be able to use a container instead: https://playwright.dev/docs/docker#remote-connection
//
// In that case you may need to set the `-playwright` flag to the container's URL, and specify the `--host` the run-server command listens on.
package test

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

var (
	anubisServer          = flag.String("anubis", "http://localhost:8923", "Anubis server URL")
	serverBindAddr        = flag.String("bind", "localhost:3923", "test server bind address")
	playwrightServer      = flag.String("playwright", "ws://localhost:3000", "Playwright server URL")
	playwrightMaxTime     = flag.Duration("playwright-max-time", 5*time.Second, "maximum time for Playwright requests")
	playwrightMaxHardTime = flag.Duration("playwright-max-hard-time", 5*time.Minute, "maximum time for hard Playwright requests")

	testCases = []testCase{
		{name: "firefox", action: actionChallenge, realIP: placeholderIP, userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:136.0) Gecko/20100101 Firefox/136.0"},
		{name: "headlessChrome", action: actionDeny, realIP: placeholderIP, userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0.6099.28 Safari/537.36"},
		{name: "kagiBadIP", action: actionChallenge, isHard: true, realIP: placeholderIP, userAgent: "Mozilla/5.0 (compatible; Kagibot/1.0; +https://kagi.com/bot)"},
		{name: "kagiGoodIP", action: actionAllow, realIP: "216.18.205.234", userAgent: "Mozilla/5.0 (compatible; Kagibot/1.0; +https://kagi.com/bot)"},
		{name: "unknownAgent", action: actionAllow, realIP: placeholderIP, userAgent: "AnubisTest/0"},
	}
)

const (
	actionAllow     action = "ALLOW"
	actionDeny      action = "DENY"
	actionChallenge action = "CHALLENGE"

	placeholderIP = "fd11:5ee:bad:c0de::"
)

type action string

type testCase struct {
	name              string
	action            action
	isHard            bool
	realIP, userAgent string
}

func TestPlaywrightBrowser(t *testing.T) {
	pw := setupPlaywright(t)
	spawnTestServer(t)
	browsers := []playwright.BrowserType{pw.Chromium, pw.Firefox, pw.WebKit}

	for _, typ := range browsers {
		for _, tc := range testCases {
			name := fmt.Sprintf("%s@%s", tc.name, typ.Name())
			t.Run(name, func(t *testing.T) {
				_, hasDeadline := t.Deadline()
				if tc.isHard && hasDeadline {
					t.Skip("skipping hard challenge with deadline")
				}

				perfomedAction := executeTestCase(t, tc, typ)

				if perfomedAction != tc.action {
					t.Errorf("unexpected test result, expected %s, got %s", tc.action, perfomedAction)
				} else {
					t.Logf("test passed")
				}
			})
		}
	}
}

func buildBrowserConnect(name string) string {
	u, _ := url.Parse(*playwrightServer)

	q := u.Query()
	q.Set("browser", name)
	u.RawQuery = q.Encode()

	return u.String()
}

func executeTestCase(t *testing.T, tc testCase, typ playwright.BrowserType) action {
	deadline, _ := t.Deadline()

	browser, err := typ.Connect(buildBrowserConnect(typ.Name()), playwright.BrowserTypeConnectOptions{
		ExposeNetwork: playwright.String("<loopback>"),
	})
	if err != nil {
		t.Fatalf("could not connect to remote browser: %v", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		AcceptDownloads: playwright.Bool(false),
		ExtraHttpHeaders: map[string]string{
			"X-Real-Ip": tc.realIP,
		},
		UserAgent: playwright.String(tc.userAgent),
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

	// Attempt challenge.

	start := time.Now()
	_, err = page.Goto(*anubisServer, playwright.PageGotoOptions{
		Timeout: pwTimeout(tc, deadline),
	})
	if err != nil {
		pwFail(t, page, "could not navigate to test server: %v", err)
	}

	hadChallenge := false
	switch tc.action {
	case actionChallenge:
		// FIXME: This could race if challenge is completed too quickly.
		checkImage(t, tc, deadline, page, "#image[src*=pensive], #image[src*=happy]")
		hadChallenge = true
	case actionDeny:
		checkImage(t, tc, deadline, page, "#image[src*=sad]")
		return actionDeny
	}

	// Ensure protected resource was provided.

	res, err := page.Locator("#anubis-test").TextContent(playwright.LocatorTextContentOptions{
		Timeout: pwTimeout(tc, deadline),
	})
	end := time.Now()
	if err != nil {
		pwFail(t, page, "could not get text content: %v", err)
	}

	var tm int64
	if _, err := fmt.Sscanf(res, "%d", &tm); err != nil {
		pwFail(t, page, "unexpected output: %s", res)
	}

	if tm < start.Unix() || end.Unix() < tm {
		pwFail(t, page, "unexpected timestamp in output: %d not in range %d..%d", tm, start.Unix(), end.Unix())
	}

	if hadChallenge {
		return actionChallenge
	} else {
		return actionAllow
	}
}

func checkImage(t *testing.T, tc testCase, deadline time.Time, page playwright.Page, locator string) {
	image := page.Locator(locator)
	err := image.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: pwTimeout(tc, deadline),
	})
	if err != nil {
		pwFail(t, page, "could not wait for result: %v", err)
	}

	failIsVisible, err := image.IsVisible()
	if err != nil {
		pwFail(t, page, "could not check result image: %v", err)
	}

	if !failIsVisible {
		pwFail(t, page, "expected result image not visible")
	}
}

func pwFail(t *testing.T, page playwright.Page, format string, args ...any) {
	t.Helper()

	saveScreenshot(t, page)
	t.Fatalf(format, args...)
}

func pwTimeout(tc testCase, deadline time.Time) *float64 {
	max := *playwrightMaxTime
	if tc.isHard {
		max = *playwrightMaxHardTime
	}

	d := deadline.Sub(time.Now())
	if d <= 0 || d > max {
		return playwright.Float(float64(max.Milliseconds()))
	}
	return playwright.Float(float64(d.Milliseconds()))
}

func saveScreenshot(t *testing.T, page playwright.Page) {
	t.Helper()

	data, err := page.Screenshot()
	if err != nil {
		t.Logf("could not take screenshot: %v", err)
		return
	}

	f, err := os.CreateTemp("", "anubis-test-fail-*.png")
	if err != nil {
		t.Logf("could not create temporary file: %v", err)
		return
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		t.Logf("could not write screenshot: %v", err)
		return
	}

	t.Logf("screenshot saved to %s", f.Name())
}

func setupPlaywright(t *testing.T) *playwright.Playwright {
	err := playwright.Install(&playwright.RunOptions{
		SkipInstallBrowsers: true,
	})
	if err != nil {
		t.Fatalf("could not install Playwright: %v", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("could not start Playwright: %v", err)
	}
	return pw
}

func spawnTestServer(t *testing.T) {
	t.Helper()

	s := new(http.Server)
	s.Addr = *serverBindAddr
	s.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><span id=anubis-test>%d</span></body></html>", time.Now().Unix())
	})

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("test HTTP server terminated unexpectedly: %v", err)
		}
	}()

	t.Cleanup(func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Fatalf("could not shutdown test server: %v", err)
		}
	})
}
