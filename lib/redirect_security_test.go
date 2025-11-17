package lib

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestRedirectSecurity(t *testing.T) {
	tests := []struct {
		reqHost  string
		testType string // "constructRedirectURL", "serveHTTPNext", "renderIndex"

		// For constructRedirectURL tests
		xForwardedProto string
		xForwardedHost  string
		xForwardedUri   string

		// For serveHTTPNext tests
		redirParam string
		name       string

		errorContains  string
		expectedStatus int

		// For renderIndex tests
		returnHTTPStatusOnly bool
		shouldError          bool
		shouldNotRedirect    bool
		shouldBlock          bool
	}{
		// constructRedirectURL tests - X-Forwarded-Proto validation
		{
			name:            "constructRedirectURL: javascript protocol should be rejected",
			testType:        "constructRedirectURL",
			xForwardedProto: "javascript",
			xForwardedHost:  "example.com",
			xForwardedUri:   "alert(1)",
			shouldError:     true,
			errorContains:   "invalid",
		},
		{
			name:            "constructRedirectURL: data protocol should be rejected",
			testType:        "constructRedirectURL",
			xForwardedProto: "data",
			xForwardedHost:  "text/html",
			xForwardedUri:   ",<script>alert(1)</script>",
			shouldError:     true,
			errorContains:   "invalid",
		},
		{
			name:            "constructRedirectURL: file protocol should be rejected",
			testType:        "constructRedirectURL",
			xForwardedProto: "file",
			xForwardedHost:  "",
			xForwardedUri:   "/etc/passwd",
			shouldError:     true,
			errorContains:   "invalid",
		},
		{
			name:            "constructRedirectURL: ftp protocol should be rejected",
			testType:        "constructRedirectURL",
			xForwardedProto: "ftp",
			xForwardedHost:  "example.com",
			xForwardedUri:   "/file.txt",
			shouldError:     true,
			errorContains:   "invalid",
		},
		{
			name:            "constructRedirectURL: https protocol should be allowed",
			testType:        "constructRedirectURL",
			xForwardedProto: "https",
			xForwardedHost:  "example.com",
			xForwardedUri:   "/foo",
			shouldError:     false,
		},
		{
			name:            "constructRedirectURL: http protocol should be allowed",
			testType:        "constructRedirectURL",
			xForwardedProto: "http",
			xForwardedHost:  "example.com",
			xForwardedUri:   "/bar",
			shouldError:     false,
		},

		// serveHTTPNext tests - redir parameter validation
		{
			name:              "serveHTTPNext: javascript: URL should be rejected",
			testType:          "serveHTTPNext",
			redirParam:        "javascript:alert(1)",
			reqHost:           "example.com",
			expectedStatus:    http.StatusBadRequest,
			shouldNotRedirect: true,
		},
		{
			name:              "serveHTTPNext: data: URL should be rejected",
			testType:          "serveHTTPNext",
			redirParam:        "data:text/html,<script>alert(1)</script>",
			reqHost:           "example.com",
			expectedStatus:    http.StatusBadRequest,
			shouldNotRedirect: true,
		},
		{
			name:              "serveHTTPNext: file: URL should be rejected",
			testType:          "serveHTTPNext",
			redirParam:        "file:///etc/passwd",
			reqHost:           "example.com",
			expectedStatus:    http.StatusBadRequest,
			shouldNotRedirect: true,
		},
		{
			name:              "serveHTTPNext: vbscript: URL should be rejected",
			testType:          "serveHTTPNext",
			redirParam:        "vbscript:msgbox(1)",
			reqHost:           "example.com",
			expectedStatus:    http.StatusBadRequest,
			shouldNotRedirect: true,
		},
		{
			name:           "serveHTTPNext: valid https URL should work",
			testType:       "serveHTTPNext",
			redirParam:     "https://example.com/foo",
			reqHost:        "example.com",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "serveHTTPNext: valid relative URL should work",
			testType:       "serveHTTPNext",
			redirParam:     "/foo/bar",
			reqHost:        "example.com",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "serveHTTPNext: external domain should be blocked",
			testType:       "serveHTTPNext",
			redirParam:     "https://evil.com/phishing",
			reqHost:        "example.com",
			expectedStatus: http.StatusBadRequest,
			shouldBlock:    true,
		},
		{
			name:           "serveHTTPNext: relative path should work",
			testType:       "serveHTTPNext",
			redirParam:     "/safe/path",
			reqHost:        "example.com",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "serveHTTPNext: empty redir should show success page",
			testType:       "serveHTTPNext",
			redirParam:     "",
			reqHost:        "example.com",
			expectedStatus: http.StatusOK,
		},

		// renderIndex tests - full subrequest auth flow
		{
			name:                 "renderIndex: javascript protocol in X-Forwarded-Proto",
			testType:             "renderIndex",
			xForwardedProto:      "javascript",
			xForwardedHost:       "example.com",
			xForwardedUri:        "alert(1)",
			returnHTTPStatusOnly: true,
			expectedStatus:       http.StatusBadRequest,
		},
		{
			name:                 "renderIndex: data protocol in X-Forwarded-Proto",
			testType:             "renderIndex",
			xForwardedProto:      "data",
			xForwardedHost:       "example.com",
			xForwardedUri:        "text/html,<script>alert(1)</script>",
			returnHTTPStatusOnly: true,
			expectedStatus:       http.StatusBadRequest,
		},
		{
			name:                 "renderIndex: valid https redirect",
			testType:             "renderIndex",
			xForwardedProto:      "https",
			xForwardedHost:       "example.com",
			xForwardedUri:        "/protected/page",
			returnHTTPStatusOnly: true,
			expectedStatus:       http.StatusTemporaryRedirect,
		},
	}

	s := &Server{
		opts: Options{
			PublicUrl:       "https://anubis.example.com",
			RedirectDomains: []string{},
		},
		logger: slog.Default(),
		policy: &policy.ParsedConfig{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.testType {
			case "constructRedirectURL":
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Forwarded-Proto", tt.xForwardedProto)
				req.Header.Set("X-Forwarded-Host", tt.xForwardedHost)
				req.Header.Set("X-Forwarded-Uri", tt.xForwardedUri)

				redirectURL, err := s.constructRedirectURL(req)

				if tt.shouldError {
					if err == nil {
						t.Errorf("expected error containing %q, got nil", tt.errorContains)
						t.Logf("got redirect URL: %s", redirectURL)
					} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
						t.Logf("expected error containing %q, got: %v", tt.errorContains, err)
					}
				} else {
					if err != nil {
						t.Errorf("expected no error, got: %v", err)
					}
					// Verify the redirect URL is safe
					if redirectURL != "" {
						parsed, err := url.Parse(redirectURL)
						if err != nil {
							t.Errorf("failed to parse redirect URL: %v", err)
						}
						redirParam := parsed.Query().Get("redir")
						if redirParam != "" {
							redirParsed, err := url.Parse(redirParam)
							if err != nil {
								t.Errorf("failed to parse redir parameter: %v", err)
							}
							if redirParsed.Scheme != "http" && redirParsed.Scheme != "https" {
								t.Errorf("redir parameter has unsafe scheme: %s", redirParsed.Scheme)
							}
						}
					}
				}

			case "serveHTTPNext":
				req := httptest.NewRequest("GET", "/.within.website/?redir="+url.QueryEscape(tt.redirParam), nil)
				req.Host = tt.reqHost
				req.URL.Host = tt.reqHost
				rr := httptest.NewRecorder()

				s.ServeHTTPNext(rr, req)

				if rr.Code != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
					t.Logf("body: %s", rr.Body.String())
				}

				if tt.shouldNotRedirect {
					location := rr.Header().Get("Location")
					if location != "" {
						t.Errorf("expected no redirect, but got Location header: %s", location)
					}
				}

				if tt.shouldBlock {
					location := rr.Header().Get("Location")
					if location != "" && strings.Contains(location, "evil.com") {
						t.Errorf("redirect to evil.com was not blocked: %s", location)
					}
				}

			case "renderIndex":
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Forwarded-Proto", tt.xForwardedProto)
				req.Header.Set("X-Forwarded-Host", tt.xForwardedHost)
				req.Header.Set("X-Forwarded-Uri", tt.xForwardedUri)

				rr := httptest.NewRecorder()
				s.RenderIndex(rr, req, policy.CheckResult{}, nil, tt.returnHTTPStatusOnly)

				if rr.Code != tt.expectedStatus {
					t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
				}

				if tt.expectedStatus == http.StatusTemporaryRedirect {
					location := rr.Header().Get("Location")
					if location == "" {
						t.Error("expected Location header, got none")
					} else {
						// Verify the location doesn't contain javascript:
						if strings.Contains(location, "javascript") {
							t.Errorf("Location header contains 'javascript': %s", location)
						}
					}
				}
			}
		})
	}
}
