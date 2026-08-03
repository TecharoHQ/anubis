package headerexists

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
)

func TestFactoryIsCheckerFactory(t *testing.T) {
	if _, ok := (any(Factory{})).(checker.Factory); !ok {
		t.Fatal("Factory is not an instance of checker.Factory")
	}
}

func TestImplIsCheckerImpl(t *testing.T) {
	if _, ok := (any(&Impl{})).(checker.Impl); !ok {
		t.Fatal("Impl is not an instance of checker.Impl")
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
				"header": "User-Agent"
}`),
		},
		{
			name: "not json",
			data: []byte(`]`),
			err:  config.ErrUnparseableConfig,
		},
		{
			name: "no header name",
			data: []byte(`{
  "header": ""
}`),
			err: ErrNoHeader,
		},
		{
			name: "header not canonical",
			data: []byte(`{
				"header": "user-agent"
}`),
			err: ErrHeaderNotCanonical,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(tt.data)

			if err := f.ValidateConfig(t.Context(), data); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Fatal("validation didn't do what was expected")
			}
		})
	}
}

func jsonOf(t *testing.T, inp Config) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(inp)
	if err != nil {
		t.Fatalf("can't marshal Config, this is a bug: %v", err)
	}

	return json.RawMessage(data)
}

func TestEndToEnd(t *testing.T) {
	f := Factory{}

	for _, tt := range []struct {
		name          string
		fc            Config
		header, value string
		match         bool
		err           error
	}{
		{
			name: "header present",
			fc: Config{
				Header: "User-Agent",
			},
			header: "User-Agent",
			value:  "Mozilla/5.0",
			match:  true,
			err:    nil,
		},
		{
			name: "header absent",
			fc: Config{
				Header: "User-Agent",
			},
			header: "Host",
			value:  "techaro.lol",
			match:  false,
			err:    nil,
		},
		{
			name: "header present but empty",
			fc: Config{
				Header: "User-Agent",
			},
			header: "User-Agent",
			value:  "",
			match:  false,
			err:    nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl, err := f.Create(t.Context(), jsonOf(t, tt.fc))
			if err != nil {
				t.Fatalf("can't build impl: %v", err)
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}

			r.Header.Set(tt.header, tt.value)

			t.Log(impl.Hash())
			match, err := impl.Check(r)

			if tt.match != match {
				t.Errorf("match: %v, wanted: %v", match, tt.match)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}
