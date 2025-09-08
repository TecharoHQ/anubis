package policy

import (
	"errors"
	"net/http"
	"testing"
)

func TestRemoteAddrChecker(t *testing.T) {
	for _, tt := range []struct {
		err   error
		name  string
		ip    string
		cidrs []string
		ok    bool
	}{
		{
			name:  "match_ipv4",
			cidrs: []string{"0.0.0.0/0"},
			ip:    "1.1.1.1",
			ok:    true,
			err:   nil,
		},
		{
			name:  "match_ipv6",
			cidrs: []string{"::/0"},
			ip:    "cafe:babe::",
			ok:    true,
			err:   nil,
		},
		{
			name:  "not_match_ipv4",
			cidrs: []string{"1.1.1.1/32"},
			ip:    "1.1.1.2",
			ok:    false,
			err:   nil,
		},
		{
			name:  "not_match_ipv6",
			cidrs: []string{"cafe:babe::/128"},
			ip:    "cafe:babe:4::/128",
			ok:    false,
			err:   nil,
		},
		{
			name:  "no_ip_set",
			cidrs: []string{"::/0"},
			ok:    false,
			err:   ErrMisconfiguration,
		},
		{
			name:  "invalid_ip",
			cidrs: []string{"::/0"},
			ip:    "According to all natural laws of aviation",
			ok:    false,
			err:   ErrMisconfiguration,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rac, err := NewRemoteAddrChecker(tt.cidrs)
			if err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("creating RemoteAddrChecker failed: %v", err)
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			if tt.ip != "" {
				r.Header.Add("X-Real-Ip", tt.ip)
			}

			ok, err := rac.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}

func TestHeaderMatchesChecker(t *testing.T) {
	for _, tt := range []struct {
		err            error
		name           string
		header         string
		rexStr         string
		reqHeaderKey   string
		reqHeaderValue string
		ok             bool
	}{
		{
			name:           "match",
			header:         "Cf-Worker",
			rexStr:         ".*",
			reqHeaderKey:   "Cf-Worker",
			reqHeaderValue: "true",
			ok:             true,
			err:            nil,
		},
		{
			name:           "not_match",
			header:         "Cf-Worker",
			rexStr:         "false",
			reqHeaderKey:   "Cf-Worker",
			reqHeaderValue: "true",
			ok:             false,
			err:            nil,
		},
		{
			name:           "not_present",
			header:         "Cf-Worker",
			rexStr:         "foobar",
			reqHeaderKey:   "Something-Else",
			reqHeaderValue: "true",
			ok:             false,
			err:            nil,
		},
		{
			name:   "invalid_regex",
			rexStr: "a(b",
			err:    ErrMisconfiguration,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hmc, err := NewHeaderMatchesChecker(tt.header, tt.rexStr)
			if err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("creating HeaderMatchesChecker failed")
			}

			if tt.err != nil && hmc == nil {
				return
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			r.Header.Set(tt.reqHeaderKey, tt.reqHeaderValue)

			ok, err := hmc.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}

func TestHeaderExistsChecker(t *testing.T) {
	for _, tt := range []struct {
		name      string
		header    string
		reqHeader string
		ok        bool
	}{
		{
			name:      "match",
			header:    "Authorization",
			reqHeader: "Authorization",
			ok:        true,
		},
		{
			name:      "not_match",
			header:    "Authorization",
			reqHeader: "Authentication",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hec := headerExistsChecker{tt.header}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			r.Header.Set(tt.reqHeader, "hunter2")

			ok, err := hec.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil {
				t.Errorf("err: %v", err)
			}
		})
	}
}

func TestPathChecker_XOriginalURI(t *testing.T) {
	tests := []struct {
		name          string
		regex         string
		xOriginalURI  string
		urlPath       string
		headerKey     string
		expectedMatch bool
		expectError   bool
	}{
		{
			name:          "X-Original-URI matches regex (with trailing space - current typo)",
			regex:         "^/api/.*",
			xOriginalURI:  "/api/users",
			urlPath:       "/different/path",
			headerKey:     "X-Original-URI",
			expectedMatch: true,
			expectError:   false,
		},
		{
			name:          "X-Original-URI doesn't match, falls back to URL.Path",
			regex:         "^/admin/.*",
			xOriginalURI:  "/api/users",
			urlPath:       "/admin/dashboard",
			headerKey:     "X-Original-URI",
			expectedMatch: true,
			expectError:   false,
		},
		{
			name:          "Neither X-Original-URI nor URL.Path match",
			regex:         "^/admin/.*",
			xOriginalURI:  "/api/users",
			urlPath:       "/public/info",
			headerKey:     "X-Original-URI ",
			expectedMatch: false,
			expectError:   false,
		},
		{
			name:          "Empty X-Original-URI, URL.Path matches",
			regex:         "^/static/.*",
			xOriginalURI:  "",
			urlPath:       "/static/css/style.css",
			headerKey:     "X-Original-URI",
			expectedMatch: true,
			expectError:   false,
		},
		{
			name:          "Complex regex matching X-Original-URI",
			regex:         `^/api/v[0-9]+/(users|posts)/[0-9]+$`,
			xOriginalURI:  "/api/v1/users/123",
			urlPath:       "/different",
			headerKey:     "X-Original-URI",
			expectedMatch: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the PathChecker
			pc, err := NewPathChecker(tt.regex)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("NewPathChecker() unexpected error: %v", err)
				}
				return
			}

			if tt.expectError {
				t.Fatal("NewPathChecker() expected error but got none")
			}

			req, err := http.NewRequest("GET", "http://example.com"+tt.urlPath, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			if tt.xOriginalURI != "" {
				req.Header.Set(tt.headerKey, tt.xOriginalURI)
			}

			match, err := pc.Check(req)
			if err != nil {
				t.Fatalf("Check() unexpected error: %v", err)
			}

			if match != tt.expectedMatch {
				t.Errorf("Check() = %v, want %v", match, tt.expectedMatch)
			}
		})
	}
}
