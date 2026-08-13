package secchua

import (
	"errors"
	"fmt"
	"testing"
)

func TestClientEquals(t *testing.T) {
	for _, tt := range []struct {
		name     string
		lhs, rhs Client
		err      error
	}{
		{"basic eq", Client{}, Client{}, nil},
		{
			name: "version count !=",
			lhs:  Client{Versions: []Version{{"Not=A?Brand", "99"}}},
			rhs:  Client{},
			err:  ErrClientVersionCountMismatch,
		},
		{
			name: "version !=",
			lhs:  Client{Versions: []Version{{"Google Chrome", "99"}}},
			rhs:  Client{Versions: []Version{{"Chromium", "151"}}},
			err:  ErrVersionMismatch,
		},
		{
			name: "mobile nilness",
			lhs:  Client{Mobile: nil},
			rhs:  Client{Mobile: new(true)},
			err:  ErrClientMobileMismatch,
		},
		{
			name: "mobile mismatch",
			lhs:  Client{Mobile: new(false)},
			rhs:  Client{Mobile: new(true)},
			err:  ErrClientMobileMismatch,
		},
		{
			name: "platform mismatch",
			lhs:  Client{Platform: ""},
			rhs:  Client{Platform: "some imaginary platform"},
			err:  ErrClientPlatformMismatch,
		},
		{
			name: "arch mismatch",
			lhs:  Client{Arch: "x86"},
			rhs:  Client{Arch: "arm"},
			err:  ErrClientArchMismatch,
		},
		{
			name: "bitness mismatch",
			lhs:  Client{Bitness: "64"},
			rhs:  Client{Bitness: "32"},
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
