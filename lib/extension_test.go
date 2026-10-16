package lib

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/challenge/extension"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/a-h/templ"
)

// XXX(Xe): This is the only real place this test can run because anywhere
// else incurs wicked import loops that I really don't want to trace down
// because that means a lot of drift across a lot of the program.

const testExtensionMarker = `<meta name="text-extension" />`

// testExtension is a test fixture extension that tests the extension
// loading and dispatch logic.
type testExtension struct {
	lock        sync.Mutex
	validateErr error
	seenIDs     []string
}

func (te *testExtension) Setup(mux *http.ServeMux, st store.Interface) error { return nil }

func (te *testExtension) Head(r *http.Request, chall *challenge.Challenge) templ.Component {
	return templ.Raw(testExtensionMarker)
}

func (te *testExtension) Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error {
	te.lock.Lock()
	defer te.lock.Unlock()
	te.seenIDs = append(te.seenIDs, in.Challenge.ID)
	return te.validateErr
}

func (te *testExtension) reset(validateErr error) {
	te.lock.Lock()
	defer te.lock.Unlock()
	te.validateErr = validateErr
	te.seenIDs = nil
}

func (te *testExtension) validatedIDs() []string {
	te.lock.Lock()
	defer te.lock.Unlock()
	return append([]string(nil), te.seenIDs...)
}

var theTestExtension = &testExtension{}

func init() {
	extension.Register("test-extension", theTestExtension)
}

func readChallengePageBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	var body io.Reader = resp.Body

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gz, err := gzip.NewReader(body)
		if err != nil {
			t.Fatalf("can't read gzipped response: %v", err)
		}
		defer gz.Close() //nolint:errcheck
		body = gz
	}

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("can't read challenge page: %v", err)
	}
	return string(data)
}

func authCookie(srv *Server, resp *http.Response) *http.Cookie {
	name := srv.cookieName(anubis.CookieName)
	for _, ckie := range resp.Cookies() {
		if ckie.Name == name && ckie.Value != "" {
			return ckie
		}
	}
	return nil
}

func TestChallengeExtensionValidate(t *testing.T) {
	for _, tt := range []struct {
		name        string
		validateErr error
		wantStatus  int
		wantCookie  bool
	}{
		{
			name:       "accepted",
			wantStatus: http.StatusFound,
			wantCookie: true,
		},
		{
			name:        "refused with challenge error",
			validateErr: challenge.NewError("validate", "browser failed", challenge.ErrFailed),
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "refused with plain error",
			validateErr: errors.New("store is down"),
			wantStatus:  http.StatusInternalServerError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			theTestExtension.reset(tt.validateErr)

			srv := spawnAnubis(t, Options{
				Next:   http.NewServeMux(),
				Policy: loadPolicies(t, "./testdata/challenge-extension.yaml", 0),
			})
			ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
			defer ts.Close()

			cli := httpClient(t)
			chall := makeChallenge(t, ts, cli)
			resp := handleChallengeZeroDifficulty(t, ts, cli, chall)
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("wanted status %d, got: %d", tt.wantStatus, resp.StatusCode)
			}

			if got := authCookie(srv, resp) != nil; got != tt.wantCookie {
				t.Errorf("wanted auth cookie set: %v, got: %v", tt.wantCookie, got)
			}

			if got := theTestExtension.validatedIDs(); len(got) != 1 || got[0] != chall.ID {
				t.Errorf("wanted extension to validate challenge %q once, got: %q", chall.ID, got)
			}
		})
	}
}

func TestChallengeExtensionHeadIsRendered(t *testing.T) {
	theTestExtension.reset(nil)

	srv := spawnAnubis(t, Options{
		Next:   http.NewServeMux(),
		Policy: loadPolicies(t, "./testdata/challenge-extension.yaml", 0),
	})
	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	defer ts.Close()

	cli := httpClient(t)
	resp, err := cli.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("can't get challenge page: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body := readChallengePageBody(t, resp)
	headMarker := strings.Index(body, testExtensionMarker)
	if headMarker == -1 {
		t.Fatalf("challenge page does not contain the extension head %q", testExtensionMarker)
	}

	// Extension scripts must run before the challenge script.
	mainAt := strings.Index(body, "anubis-main")
	if mainAt == -1 {
		t.Fatal("challenge page does not contain the main script")
	}
	if headMarker > mainAt {
		t.Error("extension head is rendered after the challenge script")
	}
}

func TestChallengeKnownExtensionMakesPolicyLoad(t *testing.T) {
	_, err := LoadPoliciesOrDefault(context.Background(), "./testdata/challenge-extension.yaml", 0, "info", false)
	if err != nil {
		t.Fatalf("unexpected config parsing failure: %v", err)
	}
}

func TestChallengeExtensionUnknownFailsPolicyLoad(t *testing.T) {
	_, err := LoadPoliciesOrDefault(context.Background(), "./testdata/challenge-extension-unknown.yaml", 0, "info", false)
	if !errors.Is(err, ErrUnknownChallengeExtension) {
		t.Fatalf("wanted %v, got: %v", ErrUnknownChallengeExtension, err)
	}
}
