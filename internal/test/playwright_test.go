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
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/playwright-community/playwright-go"
)

var (
	playwrightPort        = flag.Int("playwright-port", 9001, "Playwright port")
	playwrightServer      = flag.String("playwright", "ws://localhost:9001", "Playwright server URL")
	playwrightMaxTime     = flag.Duration("playwright-max-time", 5*time.Second, "maximum time for Playwright requests")
	playwrightMaxHardTime = flag.Duration("playwright-max-hard-time", 5*time.Minute, "maximum time for hard Playwright requests")

	testCases = []testCase{
		{
			name:      "firefox",
			action:    actionChallenge,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:136.0) Gecko/20100101 Firefox/136.0",
		},
		{
			name:      "headlessChrome",
			action:    actionDeny,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0.6099.28 Safari/537.36",
		},
		{
			name:      "kagiBadIP",
			action:    actionChallenge,
			isHard:    true,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 (compatible; Kagibot/1.0; +https://kagi.com/bot)",
		},
		{
			name:      "kagiGoodIP",
			action:    actionAllow,
			realIP:    "216.18.205.234",
			userAgent: "Mozilla/5.0 (compatible; Kagibot/1.0; +https://kagi.com/bot)",
		},
		{
			name:      "unknownAgent",
			action:    actionAllow,
			realIP:    placeholderIP,
			userAgent: "AnubisTest/0",
		},
	}
)

const (
	actionAllow     action = "ALLOW"
	actionDeny      action = "DENY"
	actionChallenge action = "CHALLENGE"

	placeholderIP     = "fd11:5ee:bad:c0de::"
	playwrightVersion = "1.50.1"
)

type action string

type testCase struct {
	name              string
	action            action
	isHard            bool
	realIP, userAgent string
}

func doesNPXExist(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("npx"); err != nil {
		t.Skipf("npx not found in PATH, skipping integration smoke testing: %v", err)
	}
}

func run(t *testing.T, command string) string {
	t.Helper()

	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("[unexpected] %v", err)
	}

	t.Logf("running command: %s", command)

	cmd := exec.Command(shPath, "-c", command)
	cmd.Stdin = nil
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("can't run command: %v", err)
	}

	return string(output)
}

func daemonize(t *testing.T, command string) {
	t.Helper()

	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("[unexpected] %v", err)
	}

	t.Logf("daemonizing command: %s", command)

	cmd := exec.Command(shPath, "-c", command)
	cmd.Stdin = nil
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("can't daemonize command: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
	})
}

func startPlaywright(t *testing.T) {
	t.Helper()

	if os.Getenv("CI") == "true" {
		run(t, fmt.Sprintf("npx --yes playwright@%s install --with-deps", playwrightVersion))
	} else {
		run(t, fmt.Sprintf("npx --yes playwright@%s install", playwrightVersion))
	}

	daemonize(t, fmt.Sprintf("npx --yes playwright@%s run-server --port %d", playwrightVersion, *playwrightPort))

	for {
		if _, err := http.Get(fmt.Sprintf("http://localhost:%d", *playwrightPort)); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	//nosleep:bypass XXX(Xe): Playwright doesn't have a good way to signal readiness. This is a HACK that will just let the tests pass.
	time.Sleep(2 * time.Second)
}

func TestPlaywrightBrowser(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	doesNPXExist(t)
	startPlaywright(t)

	pw := setupPlaywright(t)
	anubisURL := spawnAnubis(t)

	browsers := []playwright.BrowserType{pw.Chromium, pw.Firefox, pw.WebKit}

	for _, typ := range browsers {
		t.Run(typ.Name()+"/warmup", func(t *testing.T) {
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
					"X-Real-Ip": "127.0.0.1",
				},
				UserAgent: playwright.String("Sephiroth"),
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

			timeout := 2.0
			page.Goto(anubisURL, playwright.PageGotoOptions{
				Timeout: &timeout,
			})
		})

		for _, tc := range testCases {
			name := fmt.Sprintf("%s/%s", typ.Name(), tc.name)
			t.Run(name, func(t *testing.T) {
				_, hasDeadline := t.Deadline()
				if tc.isHard && hasDeadline {
					t.Skip("skipping hard challenge with deadline")
				}

				var perfomedAction action
				var err error
				for i := 0; i < 5; i++ {
					perfomedAction, err = executeTestCase(t, tc, typ, anubisURL)
					if perfomedAction == tc.action {
						break
					}
					time.Sleep(time.Duration(i+1) * 250 * time.Millisecond)
				}
				if perfomedAction != tc.action {
					t.Errorf("unexpected test result, expected %s, got %s", tc.action, perfomedAction)
				}
				if err != nil {
					t.Fatalf("test error: %v", err)
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

func executeTestCase(t *testing.T, tc testCase, typ playwright.BrowserType, anubisURL string) (action, error) {
	deadline, _ := t.Deadline()

	browser, err := typ.Connect(buildBrowserConnect(typ.Name()), playwright.BrowserTypeConnectOptions{
		ExposeNetwork: playwright.String("<loopback>"),
	})
	if err != nil {
		return "", fmt.Errorf("could not connect to remote browser: %w", err)
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
		return "", fmt.Errorf("could not create context: %w", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		return "", fmt.Errorf("could not create page: %w", err)
	}
	defer page.Close()

	// Attempt challenge.

	start := time.Now()
	_, err = page.Goto(anubisURL, playwright.PageGotoOptions{
		Timeout: pwTimeout(tc, deadline),
	})
	if err != nil {
		return "", pwFail(t, page, "could not navigate to test server: %v", err)
	}

	hadChallenge := false
	switch tc.action {
	case actionChallenge:
		// FIXME: This could race if challenge is completed too quickly.
		checkImage(t, tc, deadline, page, "#image[src*=pensive], #image[src*=happy]")
		hadChallenge = true
	case actionDeny:
		checkImage(t, tc, deadline, page, "#image[src*=sad]")
		return actionDeny, nil
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
		return actionChallenge, nil
	} else {
		return actionAllow, nil
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

func pwFail(t *testing.T, page playwright.Page, format string, args ...any) error {
	t.Helper()

	saveScreenshot(t, page)
	return fmt.Errorf(format, args...)
}

func pwTimeout(tc testCase, deadline time.Time) *float64 {
	max := *playwrightMaxTime
	if tc.isHard {
		max = *playwrightMaxHardTime
	}

	d := time.Until(deadline)
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

func spawnAnubis(t *testing.T) string {
	t.Helper()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><span id=anubis-test>%d</span></body></html>", time.Now().Unix())
	})

	policy, err := libanubis.LoadPoliciesOrDefault("", anubis.DefaultDifficulty)
	if err != nil {
		t.Fatal(err)
	}

	s, err := libanubis.New(libanubis.Options{
		Next:           h,
		Policy:         policy,
		ServeRobotsTXT: true,
	})
	if err != nil {
		t.Fatalf("can't construct libanubis.Server: %v", err)
	}

	ts := httptest.NewServer(s)
	t.Log(ts.URL)

	t.Cleanup(func() {
		ts.Close()
	})

	return ts.URL
}
