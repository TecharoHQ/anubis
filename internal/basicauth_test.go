package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	const (
		realm    = "test-realm"
		username = "admin"
		password = "hunter2"
	)

	for _, tt := range []struct {
		name       string
		setAuth    bool
		user       string
		pass       string
		wantStatus int
		wantBody   string
		wantChall  bool
	}{
		{
			name:       "valid credentials",
			setAuth:    true,
			user:       username,
			pass:       password,
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "missing credentials",
			setAuth:    false,
			wantStatus: http.StatusUnauthorized,
			wantChall:  true,
		},
		{
			name:       "wrong username",
			setAuth:    true,
			user:       "nobody",
			pass:       password,
			wantStatus: http.StatusUnauthorized,
			wantChall:  true,
		},
		{
			name:       "wrong password",
			setAuth:    true,
			user:       username,
			pass:       "wrong",
			wantStatus: http.StatusUnauthorized,
			wantChall:  true,
		},
		{
			name:       "empty supplied credentials",
			setAuth:    true,
			user:       "",
			pass:       "",
			wantStatus: http.StatusUnauthorized,
			wantChall:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := BasicAuth(realm, username, password, okHandler())

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}

			chall := rec.Header().Get("WWW-Authenticate")
			if tt.wantChall {
				if chall == "" {
					t.Error("WWW-Authenticate header missing on 401")
				}
				if !strings.Contains(chall, realm) {
					t.Errorf("WWW-Authenticate = %q, want realm %q", chall, realm)
				}
			} else if chall != "" {
				t.Errorf("unexpected WWW-Authenticate header: %q", chall)
			}
		})
	}
}

func TestBasicAuthPassthrough(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		username string
		password string
	}{
		{name: "both empty", username: "", password: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := BasicAuth("realm", tt.username, tt.password, okHandler())

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d (passthrough expected)", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != "ok" {
				t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
			}
		})
	}
}

func TestBasicAuthPartialCredentialsFailClosed(t *testing.T) {
	for _, tc := range []struct{ user, pass string }{{"admin", ""}, {"", "secret"}} {
		t.Run(tc.user+tc.pass, func(t *testing.T) {
			called := false
			h := BasicAuth("test", tc.user, tc.pass, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
			r := httptest.NewRequest("GET", "/", nil)
			r.SetBasicAuth(tc.user, tc.pass)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if called || w.Code != http.StatusUnauthorized {
				t.Fatalf("called=%v status=%d", called, w.Code)
			}
		})
	}
}
