package remoteaddress

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestFactoryIsCheckerFactory(t *testing.T) {
	if _, ok := (any(Factory{})).(checker.Factory); !ok {
		t.Fatal("Factory is not an instance of checker.Factory")
	}
}

func TestImplIsCheckerImpl(t *testing.T) {
	if _, ok := (any(&Impl{})).(checker.Impl); !ok {
		t.Fatal("Factory is not an instance of checker.Factory")
	}
}

func TestFactoryValidateConfig(t *testing.T) {
	f := Factory{}

	for _, tt := range []struct {
		name string
		data []byte
		err  error
	}{
		{
			name: "basic valid",
			data: []byte(`{
  "addrs": [
    "1.1.1.1/32"
  ]
}`),
		},
		{
			name: "not json",
			data: []byte(`]`),
			err:  config.ErrUnparseableConfig,
		},
		{
			name: "no cidr",
			data: []byte(`{
  "addrs": []
}`),
			err: ErrNoRemoteAddresses,
		},
		{
			name: "bad cidr",
			data: []byte(`{
  "addrs": [
    "according to all laws of aviation"
  ]
}`),
			err: config.ErrInvalidCIDR,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			if err := f.ValidateConfig(data); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Fatal("validation didn't do what was expected")
			}
		})
	}
}

func TestFactoryCreate(t *testing.T) {
	f := Factory{}

	for _, tt := range []struct {
		name  string
		data  []byte
		err   error
		ip    string
		match bool
	}{
		{
			name: "basic valid",
			data: []byte(`{
  "addrs": [
    "1.1.1.1/32"
  ]
}`),
			ip:    "1.1.1.1",
			match: true,
		},
		{
			name: "bad cidr",
			data: []byte(`{
  "addrs": [
    "according to all laws of aviation"
  ]
}`),
			err: config.ErrUnparseableConfig,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			impl, err := f.Create(data)
			if !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Fatal("creation didn't do what was expected")
			}

			if tt.err != nil {
				return
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			if tt.ip != "" {
				r.Header.Add("X-Real-Ip", tt.ip)
			}

			match, err := impl.Check(r)

			if tt.match != match {
				t.Errorf("match: %v, wanted: %v", match, tt.match)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}

			if impl.Hash() == "" {
				t.Error("hash method returns empty string")
			}
		})
	}
}

func TestRemoteAddrChecker(t *testing.T) {
	for _, tt := range []struct {
		err   error
		name  string
		ip    string
		cidrs []string
		ok    bool
	}{
		{
			name:  "match_ipv4",
			cidrs: []string{"0.0.0.0/0"},
			ip:    "1.1.1.1",
			ok:    true,
			err:   nil,
		},
		{
			name:  "match_ipv6",
			cidrs: []string{"::/0"},
			ip:    "cafe:babe::",
			ok:    true,
			err:   nil,
		},
		{
			name:  "not_match_ipv4",
			cidrs: []string{"1.1.1.1/32"},
			ip:    "1.1.1.2",
			ok:    false,
			err:   nil,
		},
		{
			name:  "not_match_ipv6",
			cidrs: []string{"cafe:babe::/128"},
			ip:    "cafe:babe:4::/128",
			ok:    false,
			err:   nil,
		},
		{
			name:  "no_ip_set",
			cidrs: []string{"::/0"},
			ok:    false,
			err:   policy.ErrMisconfiguration,
		},
		{
			name:  "invalid_ip",
			cidrs: []string{"::/0"},
			ip:    "According to all natural laws of aviation",
			ok:    false,
			err:   policy.ErrMisconfiguration,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(fileConfig{
				RemoteAddr: tt.cidrs,
			})
			if err != nil {
				t.Fatalf("creating config failed: %v", err)
			}
			rac, err := (Factory{}).Create(json.RawMessage(data))
			if err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("creating RemoteAddrChecker failed: %v", err)
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			if tt.ip != "" {
				r.Header.Add("X-Real-Ip", tt.ip)
			}

			ok, err := rac.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}
