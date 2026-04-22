package config

import (
	"errors"
	"testing"
)

func TestMetricsValid(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input *Metrics
		err   error
	}{
		{
			name: "basic TCP",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
			},
		},
		{
			name: "basic TCP4",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp4",
			},
		},
		{
			name: "basic TCP6",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp6",
			},
		},
		{
			name: "basic unix",
			input: &Metrics{
				Bind:       "/tmp/anubis-metrics.sock",
				Network:    "unix",
				SocketMode: "0770",
			},
		},
		{
			name:  "no bind",
			input: &Metrics{},
			err:   ErrNoMetricsBind,
		},
		{
			name:  "no network",
			input: &Metrics{},
			err:   ErrNoMetricsNetwork,
		},
		{
			name: "no unix socket mode",
			input: &Metrics{
				Bind:    "/tmp/anubis-metrics.sock",
				Network: "unix",
			},
			err: ErrNoMetricsSocketMode,
		},
		{
			name: "invalid unix socket mode",
			input: &Metrics{
				Bind:       "/tmp/anubis-metrics.sock",
				Network:    "unix",
				SocketMode: "taco bell",
			},
			err: ErrInvalidMetricsSocketMode,
		},
		{
			name: "invalid network",
			input: &Metrics{
				Bind:    ":9090",
				Network: "taco",
			},
			err: ErrInvalidMetricsNetwork,
		},
		{
			name: "invalid TLS config",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS:     &MetricsTLS{},
			},
			err: ErrInvalidMetricsTLSConfig,
		},
		{
			name: "selfsigned TLS cert",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls/selfsigned.crt",
					Key:         "./testdata/tls/selfsigned.key",
				},
			},
		},
		{
			name: "wrong path to selfsigned TLS cert",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls2/selfsigned.crt",
					Key:         "./testdata/tls2/selfsigned.key",
				},
			},
			err: ErrCantReadFile,
		},
		{
			name: "unparseable TLS cert",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls/invalid.crt",
					Key:         "./testdata/tls/invalid.key",
				},
			},
			err: ErrInvalidMetricsTLSKeypair,
		},
		{
			name: "mTLS with CA",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls/selfsigned.crt",
					Key:         "./testdata/tls/selfsigned.key",
					CA:          "./testdata/tls/minica.pem",
				},
			},
		},
		{
			name: "mTLS with nonexistent CA",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls/selfsigned.crt",
					Key:         "./testdata/tls/selfsigned.key",
					CA:          "./testdata/tls/nonexistent.crt",
				},
			},
			err: ErrCantReadFile,
		},
		{
			name: "mTLS with invalid CA",
			input: &Metrics{
				Bind:    ":9090",
				Network: "tcp",
				TLS: &MetricsTLS{
					Certificate: "./testdata/tls/selfsigned.crt",
					Key:         "./testdata/tls/selfsigned.key",
					CA:          "./testdata/tls/invalid.crt",
				},
			},
			err: ErrInvalidMetricsCACertificate,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("wanted error: %v", tt.err)
				t.Logf("got error:    %v", err)
				t.Error("validation failed")
			}
		})
	}
}
