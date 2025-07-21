package config

import (
	"errors"
	"testing"
)

func TestDomainValid(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input Domain
		err   error
	}{
		{
			name: "simple happy path",
			input: Domain{
				Name: "anubis.techaro.lol",
				TLS: TLS{
					Cert: "./testdata/tls/selfsigned.crt",
					Key:  "./testdata/tls/selfsigned.key",
				},
				Target:       "http://localhost:3000",
				HealthTarget: "http://localhost:9091/healthz",
			},
		},
		{
			name: "invalid domain name",
			input: Domain{
				Name: "\uFFFD.techaro.lol",
				TLS: TLS{
					Cert: "./testdata/tls/selfsigned.crt",
					Key:  "./testdata/tls/selfsigned.key",
				},
				Target:       "http://localhost:3000",
				HealthTarget: "http://localhost:9091/healthz",
			},
			err: ErrInvalidDomainName,
		},
		{
			name: "invalid tls config",
			input: Domain{
				Name: "anubis.techaro.lol",
				TLS: TLS{
					Cert: "./testdata/tls/invalid.crt",
					Key:  "./testdata/tls/invalid.key",
				},
				Target:       "http://localhost:3000",
				HealthTarget: "http://localhost:9091/healthz",
			},
			err: ErrInvalidDomainTLSConfig,
		},
		{
			name: "invalid URL",
			input: Domain{
				Name: "anubis.techaro.lol",
				TLS: TLS{
					Cert: "./testdata/tls/selfsigned.crt",
					Key:  "./testdata/tls/selfsigned.key",
				},
				Target:       "file://[::1:3000",
				HealthTarget: "file://[::1:9091/healthz",
			},
			err: ErrInvalidURL,
		},
		{
			name: "wrong URL scheme",
			input: Domain{
				Name: "anubis.techaro.lol",
				TLS: TLS{
					Cert: "./testdata/tls/selfsigned.crt",
					Key:  "./testdata/tls/selfsigned.key",
				},
				Target:       "file://localhost:3000",
				HealthTarget: "file://localhost:9091/healthz",
			},
			err: ErrInvalidURLScheme,
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
