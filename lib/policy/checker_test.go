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
			name:  "match_ipv4_in_ipv6",
			cidrs: []string{"0.0.0.0/0"},
			ip:    "::ffff:1.1.1.1",
			ok:    true,
			err:   nil,
		},
		{
			name:  "match_ipv4_in_ipv6_hex",
			cidrs: []string{"0.0.0.0/0"},
			ip:    "::ffff:101:101",
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
			// Create the PathChecker in subrequest mode so X-Original-URI is honored.
			pc, err := NewPathChecker(tt.regex, true)
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

// TestPathChecker_GHSA_6wcg_mqvh_fcvg is a regression test for
// https://github.com/TecharoHQ/anubis/security/advisories/GHSA-6wcg-mqvh-fcvg.
//
// PR https://github.com/TecharoHQ/anubis/pull/1015 added the ability for
// reverse proxies using Anubis in subrequest auth mode to look at the path
// of a request as there are many rules in the wild that rely on checking
// the path. This is how access to things like robots.txt or anything in the
// .well-known directory is unaffected by Anubis.
//
// However this logic was also enabled for non-subrequest deployments of Anubis,
// meaning that a specially crafted request could include a /.well-known/
// path in it and then get around Anubis with little effort.
//
// This fix gates the logic behind a new plumbed variable named subrequestMode
// that only fires when Anubis is running in subrequest auth mode. This
// properly contains that workaround so that the logic does not fire in
// most deployments.
func TestPathChecker_GHSA_6wcg_mqvh_fcvg(t *testing.T) {
	tests := []struct {
		name           string
		regex          string
		urlPath        string
		xOriginalURI   string
		subRequestMode bool
		want           bool
	}{
		{
			name:           "default mode ignores spoofed X-Original-URI when real path matches",
			regex:          "^/admin/.*",
			urlPath:        "/admin/secret",
			xOriginalURI:   "/public/index",
			subRequestMode: false,
			want:           true,
		},
		{
			name:           "default mode ignores spoofed X-Original-URI when real path does not match",
			regex:          "^/admin/.*",
			urlPath:        "/public/index",
			xOriginalURI:   "/admin/secret",
			subRequestMode: false,
			want:           false,
		},
		{
			name:           "default mode without X-Original-URI matches real path",
			regex:          "^/admin/.*",
			urlPath:        "/admin/dashboard",
			xOriginalURI:   "",
			subRequestMode: false,
			want:           true,
		},
		{
			name:           "subrequest mode honors X-Original-URI",
			regex:          "^/admin/.*",
			urlPath:        "/auth",
			xOriginalURI:   "/admin/secret",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "subrequest mode falls back to URL.Path when X-Original-URI does not match",
			regex:          "^/admin/.*",
			urlPath:        "/admin/dashboard",
			xOriginalURI:   "/public/index",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "subrequest mode with empty X-Original-URI uses URL.Path",
			regex:          "^/admin/.*",
			urlPath:        "/admin/dashboard",
			xOriginalURI:   "",
			subRequestMode: true,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := NewPathChecker(tt.regex, tt.subRequestMode)
			if err != nil {
				t.Fatalf("NewPathChecker(%q, %v) returned error: %v", tt.regex, tt.subRequestMode, err)
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.com"+tt.urlPath, nil)
			if err != nil {
				t.Fatalf("http.NewRequest: %v", err)
			}

			if tt.xOriginalURI != "" {
				req.Header.Set("X-Original-URI", tt.xOriginalURI)
			}

			got, err := pc.Check(req)
			if err != nil {
				t.Fatalf("Check() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Check() = %v, want %v (subRequestMode=%v, urlPath=%q, X-Original-URI=%q)",
					got, tt.want, tt.subRequestMode, tt.urlPath, tt.xOriginalURI)
			}
		})
	}
}

func TestPathChecker_XForwardedUri(t *testing.T) {
	tests := []struct {
		name           string
		regex          string
		xForwardedUri  string
		xOriginalURI   string
		urlPath        string
		subRequestMode bool
		want           bool
	}{
		{
			name:           "X-Forwarded-Uri matches regex in subrequest mode",
			regex:          "^/admin/.*",
			xForwardedUri:  "/admin/users",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "X-Forwarded-Uri with query string",
			regex:          "^/admin/.*",
			xForwardedUri:  "/admin/users?page=1",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "X-Original-URI takes priority over X-Forwarded-Uri",
			regex:          "^/admin/.*",
			xForwardedUri:  "/public/page",
			xOriginalURI:   "/admin/users",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "falls back to X-Forwarded-Uri when no X-Original-URI",
			regex:          "^/admin/.*",
			xForwardedUri:  "/admin/dashboard",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "neither header matches, url path matches",
			regex:          "^/public/.*",
			xForwardedUri:  "/admin/users",
			urlPath:        "/public/page",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "nothing matches",
			regex:          "^/admin/.*",
			xForwardedUri:  "/public/page",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           false,
		},
		{
			name:           "non-subrequest mode ignores X-Forwarded-Uri",
			regex:          "^/admin/.*",
			xForwardedUri:  "/admin/users",
			urlPath:        "/public/page",
			subRequestMode: false,
			want:           false,
		},
		{
			name:           "non-subrequest mode uses url path",
			regex:          "^/admin/.*",
			xForwardedUri:  "/public/page",
			urlPath:        "/admin/secret",
			subRequestMode: false,
			want:           true,
		},
		{
			name:           "empty X-Forwarded-Uri falls back to url path",
			regex:          "^/check$",
			urlPath:        "/check",
			subRequestMode: true,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := NewPathChecker(tt.regex, tt.subRequestMode)
			if err != nil {
				t.Fatalf("NewPathChecker(%q, %v) returned error: %v", tt.regex, tt.subRequestMode, err)
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.com"+tt.urlPath, nil)
			if err != nil {
				t.Fatalf("http.NewRequest: %v", err)
			}

			if tt.xForwardedUri != "" {
				req.Header.Set("X-Forwarded-Uri", tt.xForwardedUri)
			}
			if tt.xOriginalURI != "" {
				req.Header.Set("X-Original-URI", tt.xOriginalURI)
			}

			got, err := pc.Check(req)
			if err != nil {
				t.Fatalf("Check() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Check() = %v, want %v (subRequestMode=%v, urlPath=%q, X-Forwarded-Uri=%q, X-Original-URI=%q)",
					got, tt.want, tt.subRequestMode, tt.urlPath, tt.xForwardedUri, tt.xOriginalURI)
			}
		})
	}
}
