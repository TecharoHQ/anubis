package internal

import (
	"errors"
	"net/http"
	"net/http/httptest"
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

	XForwardedForUpdate(h).ServeHTTP(w, r)

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

	srv := httptest.NewServer(XForwardedForUpdate(h))

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
		name          string
		remoteAddr    string
		origXFFHeader string
		pref          XFFComputePreferences
		result        string
		err           error
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
