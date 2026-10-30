package internal

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestXForwardedForUpdateIgnoreUnix(t *testing.T) {
	var remoteAddr = ""
	var xff = ""

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddr = r.RemoteAddr
		xff = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	r.RemoteAddr = "@"

	w := httptest.NewRecorder()

	XForwardedForUpdate(true, h).ServeHTTP(w, r)

	if r.RemoteAddr != remoteAddr {
		t.Errorf("wanted remoteAddr to be %s, got: %s", r.RemoteAddr, remoteAddr)
	}

	if xff != "" {
		t.Error("handler added X-Forwarded-For when it should not have")
	}
}

func TestXForwardedForUpdateAddToChain(t *testing.T) {
	var xff = ""
	const expected = "1.1.1.1"

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xff = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(XForwardedForUpdate(true, h))

	r, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("X-Forwarded-For", "1.1.1.1,10.20.30.40")

	if _, err := srv.Client().Do(r); err != nil {
		t.Fatal(err)
	}

	if xff != expected {
		t.Logf("expected: %s", expected)
		t.Logf("got:      %s", xff)
		t.Error("X-Forwarded-For header was not what was expected")
	}
}

func TestComputeXFFHeader(t *testing.T) {
	for _, tt := range []struct {
		err           error
		name          string
		remoteAddr    string
		origXFFHeader string
		result        string
		pref          XFFComputePreferences
	}{
		{
			name:          "StripPrivate",
			remoteAddr:    "127.0.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1",
			pref: XFFComputePreferences{
				StripPrivate: true,
			},
			result: "1.1.1.1,127.0.0.1",
		},
		{
			name:          "StripPrivate",
			remoteAddr:    "127.0.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1",
			pref: XFFComputePreferences{
				StripPrivate: false,
			},
			result: "1.1.1.1,10.0.0.1,127.0.0.1",
		},
		{
			name:          "StripLoopback",
			remoteAddr:    "127.0.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1,127.0.0.1",
			pref: XFFComputePreferences{
				StripLoopback: true,
			},
			result: "1.1.1.1,10.0.0.1",
		},
		{
			name:          "StripCGNAT",
			remoteAddr:    "100.64.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1,100.64.0.1",
			pref: XFFComputePreferences{
				StripCGNAT: true,
			},
			result: "1.1.1.1,10.0.0.1",
		},
		{
			name:          "StripLinkLocalUnicastIPv4",
			remoteAddr:    "169.254.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1,169.254.0.1",
			pref: XFFComputePreferences{
				StripLLU: true,
			},
			result: "1.1.1.1,10.0.0.1",
		},
		{
			name:          "StripLinkLocalUnicastIPv6",
			remoteAddr:    "169.254.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1,fe80::",
			pref: XFFComputePreferences{
				StripLLU: true,
			},
			result: "1.1.1.1,10.0.0.1",
		},
		{
			name:          "Flatten",
			remoteAddr:    "127.0.0.1:80",
			origXFFHeader: "1.1.1.1,10.0.0.1,fe80::,100.64.0.1,169.254.0.1",
			pref: XFFComputePreferences{
				StripPrivate:  true,
				StripLoopback: true,
				StripCGNAT:    true,
				StripLLU:      true,
				Flatten:       true,
			},
			result: "1.1.1.1",
		},
		{
			name:          "TrimSpaces",
			remoteAddr:    "127.0.0.1:80",
			origXFFHeader: "1.1.1.1, 10.0.0.1, fe80::, 100.64.0.1, 169.254.0.1",
			pref: XFFComputePreferences{
				StripPrivate:  true,
				StripLoopback: true,
				StripCGNAT:    true,
				StripLLU:      true,
				Flatten:       true,
			},
			result: "1.1.1.1",
		},
		{
			name:       "invalid-ip-port",
			remoteAddr: "fe80::",
			err:        ErrCantSplitHostParse,
		},
		{
			name:       "invalid-remote-ip",
			remoteAddr: "anubis:80",
			err:        ErrCantParseRemoteIP,
		},
		{
			name:       "no-xff-dont-panic",
			remoteAddr: "127.0.0.1:80",
			pref: XFFComputePreferences{
				StripPrivate:  true,
				StripLoopback: true,
				StripCGNAT:    true,
				StripLLU:      true,
				Flatten:       true,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := computeXFFHeader(tt.remoteAddr, tt.origXFFHeader, tt.pref)
			if err != nil && !errors.Is(err, tt.err) {
				t.Errorf("computeXFFHeader got the wrong error, wanted %v but got: %v", tt.err, err)
			}

			if result != tt.result {
				t.Errorf("computeXFFHeader returned the wrong result, wanted %q but got: %q", tt.result, result)
			}
		})
	}
}

func TestParseTrustedProxies(t *testing.T) {
	for _, tt := range []struct {
		err      error
		name     string
		input    string
		want     []string
		wantErrs int
	}{
		{name: "empty", input: ""},
		{name: "only-separators", input: " , ,"},
		{
			name:  "prefixes-and-addresses",
			input: "10.0.0.0/8, 192.0.2.10 ,fd00::/8,2001:db8::1,",
			want:  []string{"10.0.0.0/8", "192.0.2.10/32", "fd00::/8", "2001:db8::1/128"},
		},
		{name: "host-bits-masked", input: "172.16.5.4/12", want: []string{"172.16.0.0/12"}},
		{name: "ipv4-mapped-address", input: "::ffff:10.0.0.1", want: []string{"10.0.0.1/32"}},
		{name: "ipv4-mapped-prefix", input: "::ffff:10.0.0.0/104", want: []string{"10.0.0.0/8"}},
		{name: "address-zone-dropped", input: "fe80::1%eth0", want: []string{"fe80::1/128"}},
		{name: "prefix-with-zone", input: "fe80::%eth0/64", err: ErrInvalidTrustedProxy, wantErrs: 1},
		{name: "hostname", input: "10.0.0.0/8,proxy.example", err: ErrInvalidTrustedProxy, wantErrs: 1},
		{name: "bad-octet", input: "10.0.0.256", err: ErrInvalidTrustedProxy, wantErrs: 1},
		{name: "too-many-bits", input: "10.0.0.0/33", err: ErrInvalidTrustedProxy, wantErrs: 1},
		{name: "every-error-reported", input: "proxy.example, 10.0.0.0/8, 10.0.0.0/33", err: ErrInvalidTrustedProxy, wantErrs: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTrustedProxies(tt.input)
			if !errors.Is(err, tt.err) {
				t.Fatalf("wanted error %v, got: %v", tt.err, err)
			}

			if tt.wantErrs != 0 {
				joined, ok := err.(interface{ Unwrap() []error })
				if !ok || len(joined.Unwrap()) != tt.wantErrs {
					t.Errorf("wanted %d errors, got: %v", tt.wantErrs, err)
				}
			}

			var gotStrings []string
			for _, prefix := range got {
				gotStrings = append(gotStrings, prefix.String())
			}

			if !slices.Equal(gotStrings, tt.want) {
				t.Errorf("wanted %v, got: %v", tt.want, gotStrings)
			}
		})
	}
}

func TestComputeTrustedXFFHeader(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.10.0.0/16"),
		netip.MustParsePrefix("fd00:10::/32"),
	}

	for _, tt := range []struct {
		err           error
		name          string
		remoteAddr    string
		origXFFHeader string
		result        string
	}{
		{
			name:          "private-client-kept",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "192.168.1.10",
			result:        "192.168.1.10",
		},
		{
			name:          "public-client",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "203.0.113.7",
			result:        "203.0.113.7",
		},
		{
			name:          "several-trusted-proxies",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "198.51.100.1, 172.16.0.5, 10.10.4.4",
			result:        "172.16.0.5",
		},
		{
			name:          "entries-left-of-client-ignored",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "10.10.9.9, 203.0.113.7",
			result:        "203.0.113.7",
		},
		{
			name:          "untrusted-peer-cannot-forge",
			remoteAddr:    "192.168.1.50:41000",
			origXFFHeader: "203.0.113.7",
			result:        "192.168.1.50",
		},
		{
			name:          "loopback-peer-trusted",
			remoteAddr:    "127.0.0.1:41000",
			origXFFHeader: "192.168.1.10",
			result:        "192.168.1.10",
		},
		{
			name:          "ipv6-loopback-peer-trusted",
			remoteAddr:    "[::1]:41000",
			origXFFHeader: "192.168.1.10",
			result:        "192.168.1.10",
		},
		{
			name:          "cgnat-client-kept",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "100.64.3.4",
			result:        "100.64.3.4",
		},
		{
			name:          "link-local-client-kept",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "169.254.10.20",
			result:        "169.254.10.20",
		},
		{
			name:          "cgnat-peer-not-trusted-unless-listed",
			remoteAddr:    "100.64.0.5:41000",
			origXFFHeader: "192.168.1.10",
			result:        "100.64.0.5",
		},
		{
			name:          "every-entry-trusted-keeps-left-most",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "10.10.1.1, 10.10.3.3",
			result:        "10.10.1.1",
		},
		{
			name:       "no-header-trusted-peer",
			remoteAddr: "10.10.0.2:41000",
			result:     "10.10.0.2",
		},
		{
			name:       "no-header-untrusted-peer",
			remoteAddr: "203.0.113.7:41000",
			result:     "203.0.113.7",
		},
		{
			name:          "unparseable-entry-stops",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "203.0.113.7, unknown, 10.10.5.5",
			result:        "10.10.5.5",
		},
		{
			name:          "unparseable-entry-left-of-client",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "unknown, 203.0.113.7",
			result:        "203.0.113.7",
		},
		{
			name:          "empty-entries-skipped",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "203.0.113.7, , 10.10.0.3,",
			result:        "203.0.113.7",
		},
		{
			name:          "ipv4-entry-with-port",
			remoteAddr:    "10.10.0.2:41000",
			origXFFHeader: "203.0.113.7:1234",
			result:        "203.0.113.7",
		},
		{
			name:          "ipv6-entry-with-port",
			remoteAddr:    "[fd00:10::2]:41000",
			origXFFHeader: "[2001:db8::1]:80, [fd00:10::7]:443",
			result:        "2001:db8::1",
		},
		{
			name:          "ipv4-mapped-peer",
			remoteAddr:    "[::ffff:10.10.0.2]:41000",
			origXFFHeader: "192.168.1.10",
			result:        "192.168.1.10",
		},
		{
			name:          "ipv6",
			remoteAddr:    "[fd00:10::2]:41000",
			origXFFHeader: "2001:db8::1, fd00:10::7",
			result:        "2001:db8::1",
		},
		{
			name:          "ipv6-private-client-kept",
			remoteAddr:    "[fd00:10::2]:41000",
			origXFFHeader: "fd12:3456::1",
			result:        "fd12:3456::1",
		},
		{
			name:          "unix-socket-peer-trusted",
			remoteAddr:    "@",
			origXFFHeader: "198.51.100.1, 192.168.1.10, 10.10.0.3",
			result:        "192.168.1.10",
		},
		{
			name:       "unix-socket-no-header",
			remoteAddr: "@",
			result:     "",
		},
		{
			name:          "bad-remote-addr",
			remoteAddr:    "10.10.0.2",
			origXFFHeader: "192.168.1.10",
			err:           ErrCantSplitHostParse,
		},
		{
			name:          "bad-remote-ip",
			remoteAddr:    "proxy.example:41000",
			origXFFHeader: "192.168.1.10",
			err:           ErrCantParseRemoteIP,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := computeTrustedXFFHeader(tt.remoteAddr, tt.origXFFHeader, trusted)
			if !errors.Is(err, tt.err) {
				t.Errorf("computeTrustedXFFHeader got the wrong error, wanted %v but got: %v", tt.err, err)
			}

			if result != tt.result {
				t.Errorf("computeTrustedXFFHeader returned the wrong result, wanted %q but got: %q", tt.result, result)
			}
		})
	}
}

func TestXForwardedForTrustedProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.10.0.0/16")}

	for _, tt := range []struct {
		name       string
		remoteAddr string
		xff        string
		extraXFF   string
		xRealIP    string
		trusted    []netip.Prefix
		wantXFF    string
		wantXReal  string
		wantRealIP string
	}{
		{
			// nginx sets X-Real-IP and X-Forwarded-For to $remote_addr
			name:       "proxy-sets-x-real-ip",
			remoteAddr: "10.10.0.2:41000",
			xff:        "192.168.1.10",
			xRealIP:    "192.168.1.10",
			trusted:    trusted,
			wantXFF:    "192.168.1.10",
			wantXReal:  "192.168.1.10",
		},
		{
			name:       "x-real-ip-from-private-client",
			remoteAddr: "10.10.0.2:41000",
			xff:        "192.168.1.10",
			trusted:    trusted,
			wantXFF:    "192.168.1.10",
			wantXReal:  "192.168.1.10",
			wantRealIP: "192.168.1.10",
		},
		{
			name:       "x-real-ip-from-public-client",
			remoteAddr: "10.10.0.2:41000",
			xff:        "198.51.100.1, 203.0.113.7",
			trusted:    trusted,
			wantXFF:    "203.0.113.7",
			wantXReal:  "203.0.113.7",
			wantRealIP: "203.0.113.7",
		},
		{
			name:       "untrusted-peer",
			remoteAddr: "192.168.1.50:41000",
			xff:        "203.0.113.7",
			trusted:    trusted,
			wantXFF:    "192.168.1.50",
			wantXReal:  "192.168.1.50",
			wantRealIP: "192.168.1.50",
		},
		{
			name:       "unix-socket",
			remoteAddr: "@",
			xff:        "203.0.113.7, 10.10.0.3",
			trusted:    trusted,
			wantXFF:    "203.0.113.7",
			wantXReal:  "203.0.113.7",
			wantRealIP: "203.0.113.7",
		},
		{
			// the proxy adds its own line after the one the client sent
			name:       "separate-header-lines",
			remoteAddr: "10.10.0.2:41000",
			xff:        "198.51.100.1",
			extraXFF:   "203.0.113.7",
			trusted:    trusted,
			wantXFF:    "203.0.113.7",
			wantXReal:  "203.0.113.7",
			wantRealIP: "203.0.113.7",
		},
		{
			name:       "unix-socket-no-header",
			remoteAddr: "@",
			trusted:    trusted,
		},
		{
			// an unchecked chain is never passed on
			name:       "bad-remote-addr-fails-closed",
			remoteAddr: "pipe",
			xff:        "6.6.6.6, 203.0.113.7",
			trusted:    trusted,
		},
		{
			// the behaviour XFF_TRUSTED_PROXIES exists to fix
			name:       "without-trusted-proxies-private-client-lost",
			remoteAddr: "10.10.0.2:41000",
			xff:        "192.168.1.10",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var gotXFF, gotXReal, gotRealIP string
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotXFF = r.Header.Get("X-Forwarded-For")
				gotXReal = r.Header.Get("X-Real-IP")
				if addr, ok := RealIP(r); ok {
					gotRealIP = addr.String()
				}
			})

			// same wiring as cmd/anubis/main.go
			var handler http.Handler
			if len(tt.trusted) != 0 {
				handler = XForwardedForTrustedProxies(tt.trusted, h)
			} else {
				handler = XForwardedForUpdate(true, XForwardedForToXRealIP(h))
			}

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.extraXFF != "" {
				r.Header.Add("X-Forwarded-For", tt.extraXFF)
			}
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			handler.ServeHTTP(httptest.NewRecorder(), r)

			if gotXFF != tt.wantXFF {
				t.Errorf("X-Forwarded-For: wanted %q, got: %q", tt.wantXFF, gotXFF)
			}
			if gotXReal != tt.wantXReal {
				t.Errorf("X-Real-IP: wanted %q, got: %q", tt.wantXReal, gotXReal)
			}
			if gotRealIP != tt.wantRealIP {
				t.Errorf("RealIP: wanted %q, got: %q", tt.wantRealIP, gotRealIP)
			}
		})
	}
}

func TestXForwardedForTrustedProxiesUnixSocket(t *testing.T) {
	// unix socket paths are limited to about 100 bytes, t.TempDir is too long on macOS
	dir, err := os.MkdirTemp("", "anubis-xff")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "anubis.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}

	var gotXFF, gotXReal string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		gotXReal = r.Header.Get("X-Real-IP")
	})
	trusted := []netip.Prefix{netip.MustParsePrefix("10.10.0.0/16")}
	srv := &http.Server{Handler: XForwardedForTrustedProxies(trusted, h)}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}}

	req, err := http.NewRequest(http.MethodGet, "http://anubis/", nil)
	if err != nil {
		t.Fatal(err)
	}
	// a client forged 6.6.6.6, the proxy appended the address it saw
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if gotXFF != "203.0.113.7" {
		t.Errorf("X-Forwarded-For: wanted %q, got: %q", "203.0.113.7", gotXFF)
	}
	if gotXReal != "203.0.113.7" {
		t.Errorf("X-Real-IP: wanted %q, got: %q", "203.0.113.7", gotXReal)
	}
}
