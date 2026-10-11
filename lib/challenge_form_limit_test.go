package lib

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestChallengeEndpointsLimitMultipartBodies(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("upload", "file")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 128<<10)); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"make", "pass", "forward"} {
		t.Run(endpoint, func(t *testing.T) {
			srv := spawnAnubis(t, Options{Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
			req := httptest.NewRequest("GET", "/?redir=/", bytes.NewReader(body.Bytes()))
			req.Header.Set("Content-Type", form.FormDataContentType())
			req.Header.Set("X-Real-IP", "192.0.2.1")
			req.AddCookie(&http.Cookie{Name: srv.cookieName(anubis.TestCookieName), Value: "test"})
			rec := httptest.NewRecorder()
			switch endpoint {
			case "make":
				srv.MakeChallenge(rec, req)
			case "pass":
				srv.PassChallenge(rec, req)
			case "forward":
				srv.ServeHTTPNext(rec, req)
			}
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("oversized multipart status %d", rec.Code)
			}
		})
	}
}

func TestChallengeFormControls(t *testing.T) {
	for _, tt := range []struct {
		name, contentType, body string
		want                    int
	}{
		{"query", "", "", http.StatusOK},
		{"urlencoded", "application/x-www-form-urlencoded", "redir=%2F", http.StatusOK},
		{"oversized urlencoded", "application/x-www-form-urlencoded", "redir=" + strings.Repeat("x", 128<<10), http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := spawnAnubis(t, Options{Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
			req := httptest.NewRequest("POST", "/?redir=/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			req.Header.Set("X-Real-IP", "192.0.2.1")
			rec := httptest.NewRecorder()
			srv.MakeChallenge(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestOriginUploadIsUnmodified(t *testing.T) {
	body := strings.Repeat("x", 128<<10)
	srv := spawnAnubis(t, Options{Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		if err != nil || string(got) != body {
			t.Errorf("upload changed: bytes %d, err %v", len(got), err)
		}
	})})
	req := httptest.NewRequest("POST", "/upload", strings.NewReader(body))
	srv.ServeHTTPNext(httptest.NewRecorder(), req)
}

// A reverse proxy that strips the request body for auth subrequests (for
// example nginx' auth_request) still forwards the original Content-Type. When
// that Content-Type is multipart/form-data and the body is empty,
// prepareChallengeForm must not treat the request as malformed.
func TestPrepareChallengeFormEmptyMultipartBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*http.Request)
	}{
		{"empty body", func(*http.Request) {}},
		// A chunked request (unknown length) whose body is empty must be
		// tolerated just like one that advertises a length of zero.
		{"empty body with unknown length", func(r *http.Request) { r.ContentLength = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Content-Type", "multipart/form-data; boundary=X")
			req.Header.Set("X-Real-IP", "192.0.2.1")
			tc.mutate(req)
			rec := httptest.NewRecorder()

			if !prepareChallengeForm(rec, req) {
				t.Fatalf("prepareChallengeForm rejected an empty multipart body: status %d", rec.Code)
			}
		})
	}
}

// The tolerance for empty bodies must not disable multipart parsing for
// requests that carry an unknown length (chunked) but do have a body.
func TestPrepareChallengeFormNonEmptyMultipartBodyUnknownLength(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("redir", "/somewhere"); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Real-IP", "192.0.2.1")
	req.ContentLength = -1 // simulate a chunked request
	rec := httptest.NewRecorder()

	if !prepareChallengeForm(rec, req) {
		t.Fatalf("prepareChallengeForm rejected a non-empty multipart body: status %d", rec.Code)
	}
	if got := req.FormValue("redir"); got != "/somewhere" {
		t.Fatalf("multipart body was not parsed: redir = %q", got)
	}
}
