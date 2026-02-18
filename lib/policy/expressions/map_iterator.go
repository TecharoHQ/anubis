package expressions

import (
	"errors"
	"maps"
	"reflect"
	"slices"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
)

var ErrNotImplemented = errors.New("expressions: not implemented")

type stringSliceIterator struct {
	keys []string
	idx  int
}

func (s *stringSliceIterator) Value() any {
	return s
}

func (s *stringSliceIterator) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, ErrNotImplemented
}

func (s *stringSliceIterator) ConvertToType(typeValue ref.Type) ref.Val {
	return types.NewErr("can't convert from %q to %q", types.IteratorType, typeValue)
}

func (s *stringSliceIterator) Equal(other ref.Val) ref.Val {
	return types.NewErr("can't compare %q to %q", types.IteratorType, other.Type())
}

func (s *stringSliceIterator) Type() ref.Type {
	return types.IteratorType
}

func (s *stringSliceIterator) HasNext() ref.Val {
	return types.Bool(s.idx < len(s.keys))
}

func (s *stringSliceIterator) Next() ref.Val {
	if s.HasNext() != types.True {
		return nil
	}

	val := s.keys[s.idx]
	s.idx++
	return types.String(val)
}

func newMapIterator(m map[string][]string) traits.Iterator {
	return &stringSliceIterator{
		keys: slices.Collect(maps.Keys(m)),
		idx:  0,
	}
}
