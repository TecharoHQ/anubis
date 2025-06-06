package config

import (
	"encoding/json"
	"errors"
	"slices"
)

var (
	ErrExpressionOrListMustBeStringOrObject = errors.New("config: this must be a string or an object")
	ErrExpressionEmpty                      = errors.New("config: this expression is empty")
	ErrExpressionCantHaveBoth               = errors.New("config: expression block can't contain multiple expression types")
)

type ExpressionOrList struct {
	Expression string   `json:"-"`
	All        []string `json:"all,omitempty"`
	Any        []string `json:"any,omitempty"`
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
