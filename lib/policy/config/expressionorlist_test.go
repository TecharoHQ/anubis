package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

func TestExpressionOrListMarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		err    error
		input  *ExpressionOrList
		name   string
		output []byte
	}{
		{
			name: "single expression",
			input: &ExpressionOrList{
				Expression: "true",
			},
			output: []byte(`"true"`),
			err:    nil,
		},
		{
			name: "all",
			input: &ExpressionOrList{
				All: []string{"true", "true"},
			},
			output: []byte(`{"all":["true","true"]}`),
			err:    nil,
		},
		{
			name: "all one",
			input: &ExpressionOrList{
				All: []string{"true"},
			},
			output: []byte(`"true"`),
			err:    nil,
		},
		{
			name: "any",
			input: &ExpressionOrList{
				Any: []string{"true", "false"},
			},
			output: []byte(`{"any":["true","false"]}`),
			err:    nil,
		},
		{
			name: "any one",
			input: &ExpressionOrList{
				Any: []string{"true"},
			},
			output: []byte(`"true"`),
			err:    nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.input)
			if !errors.Is(err, tt.err) {
				t.Errorf("wanted marshal error: %v but got: %v", tt.err, err)
			}

			if !bytes.Equal(result, tt.output) {
				t.Logf("wanted: %s", string(tt.output))
				t.Logf("got:    %s", string(result))
				t.Error("mismatched output")
			}
		})
	}
}

func TestExpressionOrListMarshalYAML(t *testing.T) {
	for _, tt := range []struct {
		err    error
		input  *ExpressionOrList
		name   string
		output []byte
	}{
		{
			name: "single expression",
			input: &ExpressionOrList{
				Expression: "true",
			},
			output: []byte(`"true"`),
			err:    nil,
		},
		{
			name: "all",
			input: &ExpressionOrList{
				All: []string{"true", "true"},
			},
			output: []byte(`all:
    - "true"
    - "true"`),
			err: nil,
		},
		{
			name: "all one",
			input: &ExpressionOrList{
				All: []string{"true"},
			},
			output: []byte(`"true"`),
			err:    nil,
		},
		{
			name: "any",
			input: &ExpressionOrList{
				Any: []string{"true", "false"},
			},
			output: []byte(`any:
    - "true"
    - "false"`),
			err: nil,
		},
		{
			name: "any one",
			input: &ExpressionOrList{
				Any: []string{"true"},
			},
			output: []byte(`"true"`),
			err:    nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := yaml.Marshal(tt.input)
			if !errors.Is(err, tt.err) {
				t.Errorf("wanted marshal error: %v but got: %v", tt.err, err)
			}

			result = bytes.TrimSpace(result)

			if !bytes.Equal(result, tt.output) {
				t.Logf("wanted: %q", string(tt.output))
				t.Logf("got:    %q", string(result))
				t.Error("mismatched output")
			}
		})
	}
}

func TestExpressionOrListUnmarshalJSON(t *testing.T) {
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

func TestExpressionOrListString(t *testing.T) {
	for _, tt := range []struct {
		name string
		out  string
		in   ExpressionOrList
	}{
		{
			name: "single expression",
			in: ExpressionOrList{
				Expression: "true",
			},
			out: "true",
		},
		{
			name: "all",
			in: ExpressionOrList{
				All: []string{"true"},
			},
			out: "( true )",
		},
		{
			name: "all with &&",
			in: ExpressionOrList{
				All: []string{"true", "true"},
			},
			out: "( true ) && ( true )",
		},
		{
			name: "any",
			in: ExpressionOrList{
				All: []string{"true"},
			},
			out: "( true )",
		},
		{
			name: "any with ||",
			in: ExpressionOrList{
				Any: []string{"true", "true"},
			},
			out: "( true ) || ( true )",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.in.String()
			if result != tt.out {
				t.Errorf("wanted %q, got: %q", tt.out, result)
			}
		})
	}
}
