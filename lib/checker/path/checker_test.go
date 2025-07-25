package path

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestChecker(t *testing.T) {
	fac := Factory{}

	for _, tt := range []struct {
		err     error
		name    string
		rexStr  string
		reqPath string
		ok      bool
	}{
		{
			name:    "match",
			rexStr:  "^/api/.*",
			reqPath: "/api/v1/users",
			ok:      true,
			err:     nil,
		},
		{
			name:    "not_match",
			rexStr:  "^/api/.*",
			reqPath: "/static/index.html",
			ok:      false,
			err:     nil,
		},
		{
			name:    "wildcard_match",
			rexStr:  ".*\\.json$",
			reqPath: "/data/config.json",
			ok:      true,
			err:     nil,
		},
		{
			name:    "wildcard_not_match",
			rexStr:  ".*\\.json$",
			reqPath: "/data/config.yaml",
			ok:      false,
			err:     nil,
		},
		{
			name:   "invalid_regex",
			rexStr: "a(b",
			err:    ErrInvalidRegex,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fc := fileConfig{
				Regex: tt.rexStr,
			}
			data, err := json.Marshal(fc)
			if err != nil {
				t.Fatal(err)
			}

			pc, err := fac.Build(t.Context(), json.RawMessage(data))
			if err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("creating PathChecker failed")
			}

			if tt.err != nil && pc == nil {
				return
			}

			t.Log(pc.Hash())

			r, err := http.NewRequest(http.MethodGet, tt.reqPath, nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			ok, err := pc.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}
