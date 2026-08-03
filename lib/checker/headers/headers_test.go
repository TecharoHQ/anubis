package headers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/checker/headerexists"
	"github.com/TecharoHQ/anubis/lib/checker/headermatches"
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
			name: "basic valid exists",
			data: []byte(`{
				"User-Agent": ".*"
}`),
		},
		{
			name: "basic valid matches",
			data: []byte(`{
				"User-Agent": "Mozilla.*"
}`),
		},
		{
			name: "not json",
			data: []byte(`]`),
			err:  config.ErrUnparseableConfig,
		},
		{
			name: "empty map",
			data: []byte(`{}`),
			err:  ErrNoHeaders,
		},
		{
			name: "header not canonical",
			data: []byte(`{
				"user-agent": ".*"
}`),
			err: headerexists.ErrHeaderNotCanonical,
		},
		{
			name: "matches header not canonical",
			data: []byte(`{
				"user-agent": "Mozilla.*"
}`),
			err: headermatches.ErrHeaderNotCanonical,
		},
		{
			name: "invalid regexp",
			data: []byte(`{
				"User-Agent": ")"
}`),
			err: headermatches.ErrInvalidHeaderRegexp,
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
		name    string
		c       Config
		headers map[string]string
		match   bool
		err     error
	}{
		{
			name: "exists match",
			c: Config{
				"User-Agent": ".*",
			},
			headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			match:   true,
		},
		{
			name: "exists no match",
			c: Config{
				"X-Custom-Header": ".*",
			},
			headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			match:   false,
		},
		{
			name: "matches match",
			c: Config{
				"User-Agent": "Mozilla.*",
			},
			headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			match:   true,
		},
		{
			name: "matches no match",
			c: Config{
				"User-Agent": "probably-not-malware.*",
			},
			headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			match:   false,
		},
		{
			name: "multiple headers all match",
			c: Config{
				"User-Agent": "Mozilla.*",
				"X-Real-Ip":  ".*",
			},
			headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
				"X-Real-Ip":  "1.1.1.1",
			},
			match: true,
		},
		{
			name: "multiple headers one fails",
			c: Config{
				"User-Agent": "Mozilla.*",
				"X-Real-Ip":  ".*",
			},
			headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
			},
			match: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			impl, err := f.Create(t.Context(), jsonOf(t, tt.c))
			if err != nil {
				t.Fatalf("can't build impl: %v", err)
			}

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't build request: %v", err)
			}

			for header, value := range tt.headers {
				r.Header.Set(header, value)
			}

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
