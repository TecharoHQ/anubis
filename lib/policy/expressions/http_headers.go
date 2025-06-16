package expressions

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

// HTTPHeaders is a type wrapper to expose HTTP headers into CEL programs.
type HTTPHeaders struct {
	http.Header
}

func (h HTTPHeaders) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, ErrNotImplemented
}

func (h HTTPHeaders) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.MapType:
		return h
	case types.TypeType:
		return types.MapType
	}

	return types.NewErr("can't convert from %q to %q", types.MapType, typeVal)
}

func (h HTTPHeaders) Equal(other ref.Val) ref.Val {
	return types.Bool(false) // We don't want to compare header maps
}

func (h HTTPHeaders) Type() ref.Type {
	return types.MapType
}

func (h HTTPHeaders) Value() any { return h }

func (h HTTPHeaders) Find(key ref.Val) (ref.Val, bool) {
	k, ok := key.(types.String)
	if !ok {
		return nil, false
	}

	if _, ok := h.Header[string(k)]; !ok {
		return nil, false
	}

	return types.String(strings.Join(h.Header.Values(string(k)), ",")), true
}

func (h HTTPHeaders) Contains(key ref.Val) ref.Val {
	_, ok := h.Find(key)
	return types.Bool(ok)
}

func (h HTTPHeaders) Get(key ref.Val) ref.Val {
	result, ok := h.Find(key)
	if !ok {
		return types.ValOrErr(result, "no such key: %v", key)
	}
	return result
}

func (h HTTPHeaders) Iterator() traits.Iterator { panic("TODO(Xe): implement me") }

func (h HTTPHeaders) IsZeroValue() bool {
	return len(h.Header) == 0
}

func (h HTTPHeaders) Size() ref.Val { return types.Int(len(h.Header)) }
