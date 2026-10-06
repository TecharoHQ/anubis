package iplist

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantCT  string
		wantErr bool
	}{
		{
			// Published lists (OpenAI, Google, ...) put one address family
			// per prefixes[] object, so both IPv4 and IPv6 are collected.
			name: "openai-style",
			input: `{
				"creationTime": "2025-01-02T03:04:05.000000",
				"prefixes": [
					{"ipv4Prefix": "20.42.10.176/28"},
					{"ipv6Prefix": "2a01:111:f403:2::/64"}
				]
			}`,
			want:   []string{"20.42.10.176/28", "2a01:111:f403:2::/64"},
			wantCT: "2025-01-02T03:04:05.000000",
		},
		{
			name:    "empty prefixes",
			input:   `{"creationTime": "2025-01-02T03:04:05.000000", "prefixes": []}`,
			want:    nil,
			wantCT:  "2025-01-02T03:04:05.000000",
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{not json`,
			wantErr: true,
		},
		{
			name:    "empty body",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "skips objects with no prefixes",
			input:   `{"creationTime": "t", "prefixes": [{}, {"ipv4Prefix": "1.1.1.1/32"}]}`,
			want:    []string{"1.1.1.1/32"},
			wantCT:  "t",
			wantErr: false,
		},
		{
			// One prefixes[] object with both fields set is not used by the
			// published format. CIDRs() treats the fields as exclusive and
			// keeps ipv4Prefix when both are present (same as iplist2rule).
			name:    "both prefixes prefers ipv4",
			input:   `{"creationTime": "t", "prefixes": [{"ipv4Prefix": "8.8.8.8/32", "ipv6Prefix": "2001:4860:4860::8888/128"}]}`,
			want:    []string{"8.8.8.8/32"},
			wantCT:  "t",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if pl.CreationTime != tt.wantCT {
				t.Errorf("CreationTime = %q, want %q", pl.CreationTime, tt.wantCT)
			}
			got := pl.CIDRs()
			if !slices.Equal(got, tt.want) {
				t.Errorf("CIDRs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseNilReader(t *testing.T) {
	_, err := Parse(io.NopCloser(strings.NewReader("")))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "can't decode prefix list") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCIDRsNilReceiver(t *testing.T) {
	var pl *PrefixList
	if got := pl.CIDRs(); got != nil {
		t.Errorf("nil receiver CIDRs() = %v, want nil", got)
	}
}
