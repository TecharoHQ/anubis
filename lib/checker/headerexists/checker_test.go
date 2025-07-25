package headerexists

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestChecker(t *testing.T) {
	fac := Factory{}

	for _, tt := range []struct {
		name      string
		header    string
		reqHeader string
		ok        bool
	}{
		{
			name:      "match",
			header:    "Authorization",
			reqHeader: "Authorization",
			ok:        true,
		},
		{
			name:      "not_match",
			header:    "Authorization",
			reqHeader: "Authentication",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hec, err := fac.Build(t.Context(), json.RawMessage(fmt.Sprintf("%q", tt.header)))
			if err != nil {
				t.Fatal(err)
			}

			t.Log(hec.Hash())

			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			r.Header.Set(tt.reqHeader, "hunter2")

			ok, err := hec.Check(r)

			if tt.ok != ok {
				t.Errorf("ok: %v, wanted: %v", ok, tt.ok)
			}

			if err != nil {
				t.Errorf("err: %v", err)
			}
		})
	}
}
