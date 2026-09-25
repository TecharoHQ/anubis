package policy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/gaissmai/bart"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newTestRemoteAddressesURLChecker(url string) *RemoteAddressesURLChecker {
	return &RemoteAddressesURLChecker{
		url:             url,
		logger:          discardLogger(),
		client:          &http.Client{Timeout: 5 * time.Second},
		prefixTable:     new(bart.Lite),
		refreshInterval: 24 * time.Hour,
		retryInterval:   15 * time.Minute,
		maxRetries:      5,
		stopped:         make(chan struct{}),
	}
}

func checkIP(t *testing.T, c *RemoteAddressesURLChecker, ip string) bool {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatalf("can't make request: %v", err)
	}
	r.Header.Set("X-Real-IP", ip)
	ok, err := c.Check(r)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return ok
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

const samplePrefixList = `{
	"creationTime": "2025-01-02T03:04:05.000000",
	"prefixes": [
		{"ipv4Prefix": "20.42.10.176/28"},
		{"ipv6Prefix": "2001:db8::/32"}
	]
}`

func TestRemoteAddressesURLChecker_emptyBeforeFetch(t *testing.T) {
	c := newTestRemoteAddressesURLChecker("http://127.0.0.1:1/missing.json")
	if checkIP(t, c, "20.42.10.176") {
		t.Fatal("empty list should not match any IP")
	}
	if checkIP(t, c, "1.1.1.1") {
		t.Fatal("empty list should not match any IP")
	}
}

func TestRemoteAddressesURLChecker_successfulFetch(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, samplePrefixList)
	}))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	delay := c.refreshOnce(t.Context())
	if delay != c.refreshInterval {
		t.Fatalf("delay = %v, want %v", delay, c.refreshInterval)
	}
	wantUA := "TecharoHQ-Anubis/" + anubis.Version
	if gotUA != wantUA {
		t.Fatalf("User-Agent = %q, want %q", gotUA, wantUA)
	}
	if c.consecutiveFail != 0 {
		t.Fatalf("consecutiveFail = %d, want 0", c.consecutiveFail)
	}
	if c.rebuildCount != 1 {
		t.Fatalf("rebuildCount = %d, want 1", c.rebuildCount)
	}
	if c.Hash() == "" {
		t.Fatal("hash should be non-empty after a successful fetch")
	}

	if !checkIP(t, c, "20.42.10.176") {
		t.Fatal("expected 20.42.10.176 to match")
	}
	if !checkIP(t, c, "20.42.10.191") {
		t.Fatal("expected 20.42.10.191 (end of /28) to match")
	}
	if checkIP(t, c, "20.42.10.192") {
		t.Fatal("expected 20.42.10.192 to be outside the prefix")
	}
	if !checkIP(t, c, "2001:db8::1") {
		t.Fatal("expected IPv6 prefix to match")
	}
	if checkIP(t, c, "1.1.1.1") {
		t.Fatal("expected unrelated IPv4 not to match")
	}
}

func TestRemoteAddressesURLChecker_http5xx(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(http.StatusInternalServerError, "nope"))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.cycleStart = time.Now()

	delay := c.refreshOnce(t.Context())
	if delay != c.retryInterval {
		t.Fatalf("delay = %v, want retry interval %v", delay, c.retryInterval)
	}
	if c.consecutiveFail != 1 {
		t.Fatalf("consecutiveFail = %d, want 1", c.consecutiveFail)
	}
	if checkIP(t, c, "20.42.10.176") {
		t.Fatal("failed fetch must not populate the list")
	}
}

func TestRemoteAddressesURLChecker_http5xxGivesUpAfterMaxRetries(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(http.StatusBadGateway, "nope"))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.cycleStart = time.Now()
	c.consecutiveFail = 4

	delay := c.refreshOnce(t.Context())
	if delay < 23*time.Hour || delay > 24*time.Hour {
		t.Fatalf("delay = %v, want remainder of 24h window", delay)
	}
	if c.consecutiveFail != 0 {
		t.Fatalf("consecutiveFail = %d, want 0 after giving up", c.consecutiveFail)
	}
}

func TestRemoteAddressesURLChecker_http5xxKeepsExistingList(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusOK)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(status.Load()))
		if status.Load() == http.StatusOK {
			_, _ = io.WriteString(w, samplePrefixList)
		}
	}))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	if delay := c.refreshOnce(t.Context()); delay != c.refreshInterval {
		t.Fatalf("initial fetch delay = %v, want %v", delay, c.refreshInterval)
	}
	if !checkIP(t, c, "20.42.10.176") {
		t.Fatal("expected initial fetch to populate the list")
	}

	status.Store(http.StatusInternalServerError)
	c.cycleStart = time.Now()
	if delay := c.refreshOnce(t.Context()); delay != c.retryInterval {
		t.Fatalf("5xx delay = %v, want %v", delay, c.retryInterval)
	}
	if !checkIP(t, c, "20.42.10.176") {
		t.Fatal("existing list must be kept after a 5xx response")
	}
}

func TestRemoteAddressesURLChecker_http4xx(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(http.StatusNotFound, "missing"))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.cycleStart = time.Now()

	delay := c.refreshOnce(t.Context())
	if delay < 23*time.Hour || delay > 24*time.Hour {
		t.Fatalf("delay = %v, want remainder of 24h window", delay)
	}
	if c.consecutiveFail != 0 {
		t.Fatalf("consecutiveFail = %d, want 0 (4xx does not retry)", c.consecutiveFail)
	}
	if checkIP(t, c, "20.42.10.176") {
		t.Fatal("4xx fetch must not populate the list")
	}
}

func TestRemoteAddressesURLChecker_invalidJSON(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(http.StatusOK, `{not json`))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.cycleStart = time.Now()

	delay := c.refreshOnce(t.Context())
	if delay < 23*time.Hour || delay > 24*time.Hour {
		t.Fatalf("delay = %v, want remainder of 24h window", delay)
	}
	if c.rebuildCount != 0 {
		t.Fatalf("rebuildCount = %d, want 0", c.rebuildCount)
	}
}

func TestRemoteAddressesURLChecker_invalidCIDR(t *testing.T) {
	body := `{"creationTime": "t", "prefixes": [{"ipv4Prefix": "not-a-cidr"}]}`
	ts := httptest.NewServer(jsonHandler(http.StatusOK, body))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.cycleStart = time.Now()

	delay := c.refreshOnce(t.Context())
	if delay < 23*time.Hour || delay > 24*time.Hour {
		t.Fatalf("delay = %v, want remainder of 24h window", delay)
	}
	if c.rebuildCount != 0 {
		t.Fatalf("rebuildCount = %d, want 0", c.rebuildCount)
	}
}

func TestRemoteAddressesURLChecker_unchangedCreationTime(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(http.StatusOK, samplePrefixList))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	if delay := c.refreshOnce(t.Context()); delay != c.refreshInterval {
		t.Fatalf("first fetch delay = %v, want %v", delay, c.refreshInterval)
	}
	hash := c.Hash()
	if c.rebuildCount != 1 {
		t.Fatalf("rebuildCount = %d, want 1", c.rebuildCount)
	}

	if delay := c.refreshOnce(t.Context()); delay != c.refreshInterval {
		t.Fatalf("second fetch delay = %v, want %v", delay, c.refreshInterval)
	}
	if c.rebuildCount != 1 {
		t.Fatalf("rebuildCount = %d, want 1 (creationTime unchanged)", c.rebuildCount)
	}
	if c.Hash() != hash {
		t.Fatal("hash should be unchanged when creationTime matches")
	}
}

func TestRemoteAddressesURLChecker_creationTimeChangeRebuilds(t *testing.T) {
	var body atomic.Value
	body.Store(samplePrefixList)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body.Load().(string))
	}))
	t.Cleanup(ts.Close)

	c := newTestRemoteAddressesURLChecker(ts.URL)
	c.refreshOnce(t.Context())

	body.Store(`{
		"creationTime": "2026-09-18T00:00:00.000000",
		"prefixes": [{"ipv4Prefix": "1.1.1.1/32"}]
	}`)
	c.refreshOnce(t.Context())

	if c.rebuildCount != 2 {
		t.Fatalf("rebuildCount = %d, want 2", c.rebuildCount)
	}
	if !checkIP(t, c, "1.1.1.1") {
		t.Fatal("expected rebuilt list to contain 1.1.1.1")
	}
	if checkIP(t, c, "20.42.10.176") {
		t.Fatal("expected old prefixes to be replaced, not merged")
	}
}

func TestNewRemoteAddressesURLChecker_invalidURL(t *testing.T) {
	_, err := NewRemoteAddressesURLChecker(t.Context(), "not a url", discardLogger())
	if !errors.Is(err, config.ErrInvalidRemoteAddressesURL) {
		t.Fatalf("err = %v, want %v", err, config.ErrInvalidRemoteAddressesURL)
	}
}

func TestNewRemoteAddressesURLChecker_runStopsOnCancel(t *testing.T) {
	started := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-r.Context().Done()
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithCancel(t.Context())
	impl, err := NewRemoteAddressesURLChecker(ctx, ts.URL, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	c := impl.(*RemoteAddressesURLChecker)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("fetch did not start")
	}

	cancel()

	select {
	case <-c.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh goroutine did not stop after context cancel")
	}
}
