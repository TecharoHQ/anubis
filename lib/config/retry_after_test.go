package config

import "testing"

func TestValidRetryAfter(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "delay seconds", value: "120", want: true},
		{name: "HTTP date", value: "Wed, 21 Oct 2026 07:28:00 GMT", want: true},
		{name: "optional empty value", value: "", want: true},
		{name: "negative seconds", value: "-120", want: false},
		{name: "invalid text", value: "tomorrow", want: false},
		{name: "header injection", value: "120\r\nX-Injected: yes", want: false},
		{name: "non-canonical HTTP date", value: "Wed, 21 Oct 2026 07:28:00 UTC", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := validRetryAfter(tt.value); got != tt.want {
				t.Errorf("validRetryAfter(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}
