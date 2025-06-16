package expressions

import (
	"errors"
	"net/url"
	"reflect"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

var ErrNotImplemented = errors.New("expressions: not implemented")

// URLValues is a type wrapper to expose url.Values into CEL programs.
type URLValues struct {
	url.Values
}

func (u URLValues) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, ErrNotImplemented
}

func (u URLValues) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.MapType:
		return u
	case types.TypeType:
		return types.MapType
	}

	return types.NewErr("can't convert from %q to %q", types.MapType, typeVal)
}

func (u URLValues) Equal(other ref.Val) ref.Val {
	return types.Bool(false) // We don't want to compare header maps
}

func (u URLValues) Type() ref.Type {
	return types.MapType
}

func (u URLValues) Value() any { return u }

func (u URLValues) Find(key ref.Val) (ref.Val, bool) {
	k, ok := key.(types.String)
	if !ok {
		return nil, false
	}

	if _, ok := u.Values[string(k)]; !ok {
		return nil, false
	}

	return types.String(strings.Join(u.Values[string(k)], ",")), true
}

func (u URLValues) Contains(key ref.Val) ref.Val {
	_, ok := u.Find(key)
	return types.Bool(ok)
}

func (u URLValues) Get(key ref.Val) ref.Val {
	result, ok := u.Find(key)
	if !ok {
		return types.ValOrErr(result, "no such key: %v", key)
	}
	return result
}

func (u URLValues) Iterator() traits.Iterator { panic("TODO(Xe): implement me") }

func (u URLValues) IsZeroValue() bool {
	return len(u.Values) == 0
}

func (u URLValues) Size() ref.Val { return types.Int(len(u.Values)) }
