package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrExpressionOrListMustBeStringOrObject = errors.New("config: this must be a string or an object")
	ErrExpressionEmpty                      = errors.New("config: this expression is empty")
	ErrExpressionCantHaveBoth               = errors.New("config: expression block can't contain multiple expression types")
)

type ExpressionOrList struct {
	Expression string   `json:"-" yaml:"-"`
	All        []string `json:"all,omitempty" yaml:"all,omitempty"`
	Any        []string `json:"any,omitempty" yaml:"any,omitempty"`
}

func (eol ExpressionOrList) String() string {
	switch {
	case len(eol.Expression) != 0:
		return eol.Expression
	case len(eol.All) != 0:
		var sb strings.Builder
		for i, pred := range eol.All {
			if i != 0 {
				fmt.Fprintf(&sb, " && ")
			}
			fmt.Fprintf(&sb, "( %s )", pred)
		}
		return sb.String()
	case len(eol.Any) != 0:
		var sb strings.Builder
		for i, pred := range eol.Any {
			if i != 0 {
				fmt.Fprintf(&sb, " || ")
			}
			fmt.Fprintf(&sb, "( %s )", pred)
		}
		return sb.String()
	}
	panic("this should not happen")
}

func (eol ExpressionOrList) Equal(rhs *ExpressionOrList) bool {
	if eol.Expression != rhs.Expression {
		return false
	}

	if !slices.Equal(eol.All, rhs.All) {
		return false
	}

	if !slices.Equal(eol.Any, rhs.Any) {
		return false
	}

	return true
}

func (eol *ExpressionOrList) MarshalYAML() (any, error) {
	switch {
	case len(eol.All) == 1 && len(eol.Any) == 0:
		eol.Expression = eol.All[0]
		eol.All = nil
	case len(eol.Any) == 1 && len(eol.All) == 0:
		eol.Expression = eol.Any[0]
		eol.Any = nil
	}

	if eol.Expression != "" {
		return eol.Expression, nil
	}

	type RawExpressionOrList ExpressionOrList
	return RawExpressionOrList(*eol), nil
}

func (eol *ExpressionOrList) MarshalJSON() ([]byte, error) {
	switch {
	case len(eol.All) == 1 && len(eol.Any) == 0:
		eol.Expression = eol.All[0]
		eol.All = nil
	case len(eol.Any) == 1 && len(eol.All) == 0:
		eol.Expression = eol.Any[0]
		eol.Any = nil
	}

	if eol.Expression != "" {
		return json.Marshal(string(eol.Expression))
	}

	type RawExpressionOrList ExpressionOrList
	val := RawExpressionOrList(*eol)
	return json.Marshal(val)
}

func (eol *ExpressionOrList) UnmarshalJSON(data []byte) error {
	switch string(data[0]) {
	case `"`: // string
		return json.Unmarshal(data, &eol.Expression)
	case "{": // object
		type RawExpressionOrList ExpressionOrList
		var val RawExpressionOrList
		if err := json.Unmarshal(data, &val); err != nil {
			return err
		}
		eol.All = val.All
		eol.Any = val.Any

		return nil
	}

	return ErrExpressionOrListMustBeStringOrObject
}

func (eol *ExpressionOrList) Valid() error {
	if eol.Expression == "" && len(eol.All) == 0 && len(eol.Any) == 0 {
		return ErrExpressionEmpty
	}
	if len(eol.All) != 0 && len(eol.Any) != 0 {
		return ErrExpressionCantHaveBoth
	}

	return nil
}
