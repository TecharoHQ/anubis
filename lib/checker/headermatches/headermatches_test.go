package headermatches

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
				"header": "Host",
				"regexp": ".*"
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
  "regexp": ".*"
}`),
			err: ErrNoHeaderName,
		},
		{
			name: "no regexp",
			data: []byte(`{
  "header": ".*"
}`),
			err: ErrNoHeaderRegexp,
		},
		{
			name: "header not canonical",
			data: []byte(`{
				"header": "host",
				"regexp": ".*"
}`),
			err: ErrHeaderNotCanonical,
		},
		{
			name: "invalid regexp",
			data: []byte(`{
				"header": "Host",
				"regexp": ")"
}`),
			err: ErrInvalidHeaderRegexp,
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
			name: "basic host match",
			fc: Config{
				Header: "Host",
				Regexp: "techaro.lol",
			},
			header: "Host",
			value:  "techaro.lol",
			match:  true,
			err:    nil,
		},
		{
			name: "basic host not match",
			fc: Config{
				Header: "Host",
				Regexp: "probably-not-malware.lol",
			},
			header: "Host",
			value:  "techaro.lol",
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
