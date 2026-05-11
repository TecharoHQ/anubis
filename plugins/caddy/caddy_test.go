package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestUnmarshalCaddyfile(t *testing.T) {
	d := caddyfile.NewTestDispenser(`anubis {
		policy_file /etc/anubis/botPolicies.yaml
		difficulty 5
		base_prefix /protected
	}`)

	var m Anubis
	if err := m.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("UnmarshalCaddyfile returned error: %v", err)
	}

	if m.PolicyFile != "/etc/anubis/botPolicies.yaml" {
		t.Fatalf("PolicyFile = %q, want %q", m.PolicyFile, "/etc/anubis/botPolicies.yaml")
	}
	if m.Difficulty != 5 {
		t.Fatalf("Difficulty = %d, want 5", m.Difficulty)
	}
	if m.BasePrefix != "/protected" {
		t.Fatalf("BasePrefix = %q, want %q", m.BasePrefix, "/protected")
	}
}

func TestUnmarshalCaddyfileRejectsUnknownOption(t *testing.T) {
	d := caddyfile.NewTestDispenser(`anubis {
		nope
	}`)

	var m Anubis
	if err := m.UnmarshalCaddyfile(d); err == nil {
		t.Fatal("UnmarshalCaddyfile succeeded with an unknown option")
	}
}

func TestUnmarshalCaddyfileRejectsInvalidDifficulty(t *testing.T) {
	d := caddyfile.NewTestDispenser(`anubis {
		difficulty nope
	}`)

	var m Anubis
	if err := m.UnmarshalCaddyfile(d); err == nil {
		t.Fatal("UnmarshalCaddyfile succeeded with an invalid difficulty")
	}
}
