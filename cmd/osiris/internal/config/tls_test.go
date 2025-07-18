package config

import (
	"errors"
	"testing"
)

func TestTLSValid(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input TLS
		err   error
	}{
		{
			name: "simple selfsigned",
			input: TLS{
				Cert: "./testdata/tls/selfsigned.crt",
				Key:  "./testdata/tls/selfsigned.key",
			},
		},
		{
			name: "files don't exist",
			input: TLS{
				Cert: "./testdata/tls/nonexistent.crt",
				Key:  "./testdata/tls/nonexistent.key",
			},
			err: ErrCantReadTLS,
		},
		{
			name: "invalid keypair",
			input: TLS{
				Cert: "./testdata/tls/invalid.crt",
				Key:  "./testdata/tls/invalid.key",
			},
			err: ErrInvalidTLSKeypair,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong error from validation function")
			} else {
				t.Log(err)
			}
		})
	}
}
