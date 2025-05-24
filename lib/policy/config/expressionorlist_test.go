package config

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestExpressionOrListUnmarshal(t *testing.T) {
	for _, tt := range []struct {
		err      error
		validErr error
		result   *ExpressionOrList
		name     string
		inp      string
	}{
		{
			name: "simple",
			inp:  `"\"User-Agent\" in headers"`,
			result: &ExpressionOrList{
				Expression: `"User-Agent" in headers`,
			},
		},
		{
			name: "object-and",
			inp: `{
			"all": ["\"User-Agent\" in headers"]
			}`,
			result: &ExpressionOrList{
				All: []string{
					`"User-Agent" in headers`,
				},
			},
		},
		{
			name: "object-or",
			inp: `{
			"any": ["\"User-Agent\" in headers"]
			}`,
			result: &ExpressionOrList{
				Any: []string{
					`"User-Agent" in headers`,
				},
			},
		},
		{
			name: "both-or-and",
			inp: `{
			"all": ["\"User-Agent\" in headers"],
			"any": ["\"User-Agent\" in headers"]
			}`,
			validErr: ErrExpressionCantHaveBoth,
		},
		{
			name: "expression-empty",
			inp: `{
			"any": []
			}`,
			validErr: ErrExpressionEmpty,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var eol ExpressionOrList

			if err := json.Unmarshal([]byte(tt.inp), &eol); !errors.Is(err, tt.err) {
				t.Errorf("wanted unmarshal error: %v but got: %v", tt.err, err)
			}

			if tt.result != nil && !eol.Equal(tt.result) {
				t.Logf("wanted: %#v", tt.result)
				t.Logf("got:    %#v", &eol)
				t.Fatal("parsed expression is not what was expected")
			}

			if err := eol.Valid(); !errors.Is(err, tt.validErr) {
				t.Errorf("wanted validation error: %v but got: %v", tt.err, err)
			}
		})
	}
}
