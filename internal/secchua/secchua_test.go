package secchua

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseClient(t *testing.T) {
	for _, tt := range []struct {
		name     string
		buildReq func() *http.Request
		want     *Client
		err      error
	}{
		{
			name: "no sec-ch-ua",
			buildReq: func() *http.Request {
				result := httptest.NewRequest(http.MethodGet, "/", nil)
				return result
			},
			want: nil,
			err:  nil,
		},
		{
			name: "google chrome 151",
			buildReq: func() *http.Request {
				result := httptest.NewRequest(http.MethodGet, "/", nil)
				result.Header.Add("Sec-Ch-Ua", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
				result.Header.Add("Sec-Ch-Ua-Arch", `"x86"`)
				result.Header.Add("Sec-Ch-Ua-Bitness", `"64"`)
				result.Header.Add("Sec-Ch-Ua-Mobile", "?0")
				result.Header.Add("Sec-Ch-Ua-Platform", `"Linux"`)
				return result
			},
			want: &Client{
				Versions: []Version{
					{"Chromium", "151"},
					{"Google Chrome", "151"},
				},
				Mobile:   new(false),
				Platform: `"Linux"`,
				Arch:     `"x86"`,
				Bitness:  `"64"`,
			},
			err: nil,
		},
		{
			name: "invalid Sec-Ch-Ua-Mobile",
			buildReq: func() *http.Request {
				result := httptest.NewRequest(http.MethodGet, "/", nil)
				result.Header.Add("Sec-Ch-Ua", `"Google Chrome";v="151"`)
				result.Header.Add("Sec-Ch-Ua-Mobile", "yes")
				return result
			},
			want: nil,
			err:  ErrMobileIsInvalid,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClient(tt.buildReq())
			if !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf(" got: %v", err)
				t.Fatal("unexpected error result")
			}

			if err := tt.want.Equals(got); err != nil {
				t.Fatalf("can't equate clients: %v", err)
			}
		})
	}
}

func TestClientEquals(t *testing.T) {
	for _, tt := range []struct {
		name     string
		lhs, rhs *Client
		err      error
	}{
		{"basic eq", &Client{}, &Client{}, nil},
		{"both nil", nil, nil, nil},
		{
			name: "lhs nil",
			lhs:  nil,
			rhs:  &Client{},
			err:  ErrClientNilMismatch,
		},
		{
			name: "rhs nil",
			lhs:  &Client{},
			rhs:  nil,
			err:  ErrClientNilMismatch,
		},
		{
			name: "version count !=",
			lhs:  &Client{Versions: []Version{{"Not=A?Brand", "99"}}},
			rhs:  &Client{},
			err:  ErrClientVersionCountMismatch,
		},
		{
			name: "version !=",
			lhs:  &Client{Versions: []Version{{"Google Chrome", "99"}}},
			rhs:  &Client{Versions: []Version{{"Chromium", "151"}}},
			err:  ErrVersionMismatch,
		},
		{
			name: "mobile nilness",
			lhs:  &Client{Mobile: nil},
			rhs:  &Client{Mobile: new(true)},
			err:  ErrClientMobileMismatch,
		},
		{
			name: "mobile mismatch",
			lhs:  &Client{Mobile: new(false)},
			rhs:  &Client{Mobile: new(true)},
			err:  ErrClientMobileMismatch,
		},
		{
			name: "platform mismatch",
			lhs:  &Client{Platform: ""},
			rhs:  &Client{Platform: "some imaginary platform"},
			err:  ErrClientPlatformMismatch,
		},
		{
			name: "arch mismatch",
			lhs:  &Client{Arch: "x86"},
			rhs:  &Client{Arch: "arm"},
			err:  ErrClientArchMismatch,
		},
		{
			name: "bitness mismatch",
			lhs:  &Client{Bitness: "64"},
			rhs:  &Client{Bitness: "32"},
			err:  ErrClientBitnessMismatch,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.lhs.Equals(tt.rhs); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf(" got: %v", err)
				t.Fatal("unexpected equality result")
			}
		})
	}
}

func TestVersionEquals(t *testing.T) {
	for _, tt := range []struct {
		lhs, rhs Version
		err      error
	}{
		{Version{"Not=A?Brand", "99"}, Version{"Not=A?Brand", "99"}, nil},
		{Version{"Google Chrome", "151"}, Version{"Not=A?Brand", "99"}, ErrVersionMismatch},
		{Version{"Google Chrome", "151"}, Version{"Google Chrome", "151"}, nil},
	} {
		t.Run(fmt.Sprintf("Equals(%s, %s) == %v", tt.lhs, tt.rhs, tt.err), func(t *testing.T) {
			if err := tt.lhs.Equals(tt.rhs); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf(" got: %v", err)
				t.Fatal("unexpected equality result")
			}
		})
	}
}
