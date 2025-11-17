package config

import (
	"errors"
	"fmt"
	"testing"
)

func TestASNsValid(t *testing.T) {
	for _, tt := range []struct {
		err   error
		input *ASNs
		name  string
	}{
		{
			name: "basic valid",
			input: &ASNs{
				Match: []uint32{13335}, // Cloudflare
			},
		},
		{
			name: "private ASN",
			input: &ASNs{
				Match: []uint32{64513, 4206942069}, // 16 and 32 bit private ASN
			},
			err: ErrPrivateASN,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong validation error")
			}
		})
	}
}

func TestIsPrivateASN(t *testing.T) {
	for _, tt := range []struct {
		input  uint32
		output bool
	}{
		{13335, false},     // Cloudflare
		{64513, true},      // 16 bit private ASN
		{4206942069, true}, // 32 bit private ASN
	} {
		t.Run(fmt.Sprint(tt.input, "->", tt.output), func(t *testing.T) {
			result := isPrivateASN(tt.input)
			if result != tt.output {
				t.Errorf("wanted isPrivateASN(%d) == %v, got: %v", tt.input, tt.output, result)
			}
		})
	}
}
