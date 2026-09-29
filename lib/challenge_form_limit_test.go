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
