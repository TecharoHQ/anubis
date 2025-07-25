package remoteaddress_test

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/checker/remoteaddress"
)

func TestFactoryIsCheckerFactory(t *testing.T) {
	if _, ok := (any(remoteaddress.Factory{})).(checker.Factory); !ok {
		t.Fatal("Factory is not an instance of checker.Factory")
	}
}

func TestFactoryValidateConfig(t *testing.T) {
	f := remoteaddress.Factory{}

	for _, tt := range []struct {
		name string
		data []byte
		err  error
	}{
		{
			name: "basic valid",
			data: []byte(`{
  "remote_addresses": [
    "1.1.1.1/32"
  ]
}`),
		},
		{
			name: "not json",
			data: []byte(`]`),
			err:  checker.ErrUnparseableConfig,
		},
		{
			name: "no cidr",
			data: []byte(`{
  "remote_addresses": []
}`),
			err: remoteaddress.ErrNoRemoteAddresses,
		},
		{
			name: "bad cidr",
			data: []byte(`{
  "remote_addresses": [
    "according to all laws of aviation"
  ]
}`),
			err: remoteaddress.ErrInvalidCIDR,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			if err := f.Valid(t.Context(), data); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Fatal("validation didn't do what was expected")
			}
		})
	}
}

func TestFactoryCreate(t *testing.T) {
	f := remoteaddress.Factory{}

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
  "remote_addresses": [
    "1.1.1.1/32"
  ]
}`),
			ip:    "1.1.1.1",
			match: true,
		},
		{
			name: "bad cidr",
			data: []byte(`{
  "remote_addresses": [
    "according to all laws of aviation"
  ]
}`),
			err: checker.ErrUnparseableConfig,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			impl, err := f.Build(t.Context(), data)
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
