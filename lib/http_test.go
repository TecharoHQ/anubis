package lib

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRenderIndexRedirect(t *testing.T) {
	s := &Server{
		opts: Options{
			PublicUrl: "https://anubis.example.com",
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Uri", "/foo")

	rr := httptest.NewRecorder()
	s.RenderIndex(rr, req, nil, true)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rr.Code)
	}
	location := rr.Header().Get("Location")
	parsedURL, _ := url.Parse(location)

	scheme := "https"
	if parsedURL.Scheme != scheme {
		t.Errorf("expected scheme to be %q, got %q", scheme, parsedURL.Scheme)
	}

	host := "anubis.example.com"
	if parsedURL.Host != host {
		t.Errorf("expected url to be %q, got %q", host, parsedURL.Host)
	}

	redir := parsedURL.Query().Get("redir")
	expectedRedir := "https://example.com/foo"
	if redir != expectedRedir {
		t.Errorf("expected redir param to be %q, got %q", expectedRedir, redir)
	}
}

func TestRenderIndexUnauthorized(t *testing.T) {
	s := &Server{
		opts: Options{
			PublicUrl: "",
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	s.RenderIndex(rr, req, nil, true)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if body := rr.Body.String(); body != "Authorization required" {
		t.Errorf("expected body %q, got %q", "Authorization required", body)
	}
}
