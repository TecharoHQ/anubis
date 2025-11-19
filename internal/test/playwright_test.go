//go:build !windows

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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
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
	playwrightRunner      = flag.String("playwright-runner", "npx", "how to start Playwright, can be: none,npx,docker,podman")

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
			name:      "Amazonbot",
			action:    actionDeny,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/600.2.5 (KHTML, like Gecko) Version/8.0.2 Safari/600.2.5 (Amazonbot/0.1; +https://developer.amazon.com/support/amazonbot)",
		},
		{
			name:      "Amazonbot",
			action:    actionDeny,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_1) AppleWebKit/600.2.5 (KHTML, like Gecko) Version/8.0.2 Safari/600.2.5 (Amazonbot/0.1; +https://developer.amazon.com/support/amazonbot)",
		},
		{
			name:      "PerplexityAI",
			action:    actionDeny,
			realIP:    placeholderIP,
			userAgent: "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; PerplexityBot/1.0; +https://perplexity.ai/perplexitybot)",
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
	playwrightVersion = "1.52.0"
)

type action string

type testCase struct {
	name      string
	action    action
	realIP    string
	userAgent string
	isHard    bool
}

func doesCommandExist(t *testing.T, command string) {
	t.Helper()

	if _, err := exec.LookPath(command); err != nil {
		t.Skipf("%s not found in PATH, skipping integration smoke testing: %v", command, err)
	}
}

func run(t *testing.T, command string) string {
	if testing.Short() {
		t.Skip("skipping integration smoke testing in short mode")
	}
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

	if *playwrightRunner == "npx" {
		doesCommandExist(t, "npx")

		if os.Getenv("CI") == "true" {
			run(t, fmt.Sprintf("npx --yes playwright@%s install --with-deps", playwrightVersion))
		} else {
			run(t, fmt.Sprintf("npx --yes playwright@%s install", playwrightVersion))
		}

		daemonize(t, fmt.Sprintf("npx --yes playwright@%s run-server --port %d", playwrightVersion, *playwrightPort))
	} else if *playwrightRunner == "docker" || *playwrightRunner == "podman" {
		doesCommandExist(t, *playwrightRunner)

		// docs: https://playwright.dev/docs/docker
		pwcmd := fmt.Sprintf("npx -y playwright@%s run-server --port %d --host 0.0.0.0", playwrightVersion, *playwrightPort)
		container := run(t, fmt.Sprintf("%s run -d --ipc=host --user pwuser --workdir /home/pwuser --net=host mcr.microsoft.com/playwright:v%s-noble /bin/sh -c \"%s\"", *playwrightRunner, playwrightVersion, pwcmd))
		t.Cleanup(func() {
			run(t, fmt.Sprintf("%s rm --force %s", *playwrightRunner, container))
		})
	} else if *playwrightRunner == "none" {
		t.Log("not starting Playwright, assuming it is already running")
	} else {
		t.Skipf("unknown runner: %s, skipping", *playwrightRunner)
	}

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

	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("SKIP_INTEGRATION was set")
		return
	}

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

				var performedAction action
				var err error
				for i := 0; i < 5; i++ {
					performedAction, err = executeTestCase(t, tc, typ, anubisURL)
					if performedAction == tc.action {
						break
					}
					time.Sleep(time.Duration(i+1) * 250 * time.Millisecond)
				}
				if performedAction != tc.action {
					t.Errorf("unexpected test result, expected %s, got %s", tc.action, performedAction)
				}
				if err != nil {
					t.Fatalf("test error: %v", err)
				}
			})
		}
	}
}

func TestPlaywrightWithBasePrefix(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("SKIP_INTEGRATION was set")
		return
	}

	t.Skip("NOTE(Xe)\\ these tests require HTTPS support in #364")

	startPlaywright(t)

	pw := setupPlaywright(t)
	basePrefix := "/myapp"
	anubisURL := spawnAnubisWithOptions(t, basePrefix)

	// Reset BasePrefix after test
	t.Cleanup(func() {
		anubis.BasePrefix = ""
	})

	browsers := []playwright.BrowserType{pw.Chromium}

	for _, typ := range browsers {
		t.Run(typ.Name()+"/basePrefix", func(t *testing.T) {
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

			// Test accessing the base URL with prefix
			_, err = page.Goto(anubisURL+basePrefix, playwright.PageGotoOptions{
				Timeout: pwTimeout(testCases[0], time.Now().Add(5*time.Second)),
			})
			if err != nil {
				pwFail(t, page, "could not navigate to test server with base prefix: %v", err)
			}

			// Check if challenge page is displayed
			image := page.Locator("#image[src*=pensive], #image[src*=happy]")
			err = image.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: pwTimeout(testCases[0], time.Now().Add(5*time.Second)),
			})
			if err != nil {
				pwFail(t, page, "could not wait for challenge image: %v", err)
			}

			isVisible, err := image.IsVisible()
			if err != nil {
				pwFail(t, page, "could not check if challenge image is visible: %v", err)
			}
			if !isVisible {
				pwFail(t, page, "challenge image not visible")
			}

			// Complete the challenge
			// Wait for the challenge to be solved
			anubisTest := page.Locator("#anubis-test")
			err = anubisTest.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: pwTimeout(testCases[0], time.Now().Add(30*time.Second)),
			})
			if err != nil {
				pwFail(t, page, "could not wait for challenge to be solved: %v", err)
			}

			// Verify the challenge was solved
			content, err := anubisTest.TextContent(playwright.LocatorTextContentOptions{})
			if err != nil {
				pwFail(t, page, "could not get text content: %v", err)
			}

			var tm int64
			if _, err := fmt.Sscanf(content, "%d", &tm); err != nil {
				pwFail(t, page, "unexpected output: %s", content)
			}

			// Check if the timestamp is reasonable
			now := time.Now().Unix()
			if tm < now-60 || tm > now+60 {
				pwFail(t, page, "unexpected timestamp in output: %d not in range %d±60", tm, now)
			}

			// Check if cookie has the correct path
			cookies, err := ctx.Cookies()
			if err != nil {
				pwFail(t, page, "could not get cookies: %v", err)
			}

			var found bool
			for _, cookie := range cookies {
				if cookie.Name == anubis.CookieName {
					found = true
					if cookie.Path != basePrefix+"/" {
						t.Errorf("cookie path is wrong, wanted %s, got: %s", basePrefix+"/", cookie.Path)
					}
					break
				}
			}

			if !found {
				t.Errorf("Cookie %q not found", anubis.CookieName)
			}
		})
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
	maxTime := *playwrightMaxTime
	if tc.isHard {
		maxTime = *playwrightMaxHardTime
	}

	d := time.Until(deadline)
	if d <= 0 || d > maxTime {
		return playwright.Float(float64(maxTime.Milliseconds()))
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
	return spawnAnubisWithOptions(t, "")
}

func spawnAnubisWithOptions(t *testing.T, basePrefix string) string {
	t.Helper()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><span id=anubis-test>%d</span></body></html>", time.Now().Unix())
	})

	policy, err := libanubis.LoadPoliciesOrDefault(t.Context(), "", anubis.DefaultDifficulty, "info")
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("can't listen on random port: %v", err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	host := "localhost"
	port := strconv.Itoa(addr.Port)

	s, err := libanubis.New(libanubis.Options{
		Next:           h,
		Policy:         policy,
		ServeRobotsTXT: true,
		Target:         "http://" + host + ":" + port,
		BasePrefix:     basePrefix,
	})
	if err != nil {
		t.Fatalf("can't construct libanubis.Server: %v", err)
	}

	ts := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: s},
	}
	ts.Start()
	t.Log(ts.URL)

	t.Cleanup(func() {
		ts.Close()
	})

	return ts.URL
}
