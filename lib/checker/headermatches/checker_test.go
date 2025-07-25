package headermatches

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestChecker(t *testing.T) {

}

func TestHeaderMatchesChecker(t *testing.T) {
	fac := Factory{}

	for _, tt := range []struct {
		err            error
		name           string
		header         string
		rexStr         string
		reqHeaderKey   string
		reqHeaderValue string
		ok             bool
	}{
		{
			name:           "match",
			header:         "Cf-Worker",
			rexStr:         ".*",
			reqHeaderKey:   "Cf-Worker",
			reqHeaderValue: "true",
			ok:             true,
			err:            nil,
		},
		{
			name:           "not_match",
			header:         "Cf-Worker",
			rexStr:         "false",
			reqHeaderKey:   "Cf-Worker",
			reqHeaderValue: "true",
			ok:             false,
			err:            nil,
		},
		{
			name:           "not_present",
			header:         "Cf-Worker",
			rexStr:         "foobar",
			reqHeaderKey:   "Something-Else",
			reqHeaderValue: "true",
			ok:             false,
			err:            nil,
		},
		{
			name:   "invalid_regex",
			rexStr: "a(b",
			err:    ErrInvalidRegex,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fc := fileConfig{
				Header:     tt.header,
				ValueRegex: tt.rexStr,
			}
			data, err := json.Marshal(fc)
			if err != nil {
				t.Fatal(err)
			}

			hmc, err := fac.Build(t.Context(), json.RawMessage(data))
			if err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("creating HeaderMatchesChecker failed")
			}

			if tt.err != nil && hmc == nil {
				return
			}

			t.Log(hmc.Hash())

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			r.Header.Set(tt.reqHeaderKey, tt.reqHeaderValue)

			ok, err := hmc.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil && tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err: %v, wanted: %v", err, tt.err)
			}
		})
	}
}
