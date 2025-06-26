package expressions

import (
	"reflect"
	"regexp"

	"github.com/TecharoHQ/anubis/internal/fcrdns"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

var FCrDNSType *types.Type = types.NewObjectType("fcrdns")

// FCrDNS is a type to expose the FCrDNS test to CEL programs.
type FCrDNS struct {
	FCrDNS *fcrdns.FCrDNS
	Ip     string
}

func (f FCrDNS) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, ErrNotImplemented
}

func (f FCrDNS) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case FCrDNSType:
		return f
	case types.TypeType:
		return FCrDNSType
	}

	return types.NewErr("can't convert from %q to %q", FCrDNSType, typeVal)
}

func (f FCrDNS) Equal(other ref.Val) ref.Val {
	return types.Bool(false) // We don't want to compare FCrDNS instances
}

func (f FCrDNS) Type() ref.Type {
	return FCrDNSType
}

func (f FCrDNS) Value() any { return f }

func (f FCrDNS) check(pattern string) ref.Val {
	rex, err := regexp.Compile(pattern)
	if err != nil {
		return types.ValOrErr(types.False, "%w: Failed to compiler regexp pattern %s", err, pattern)
	}
	return f.checkOptimized(rex)
}

func (f FCrDNS) checkOptimized(pattern *regexp.Regexp) ref.Val {
	if f.Ip == "" {
		return types.ValOrErr(types.False, "header X-Real-Ip is not set")
	}

	res, err := f.FCrDNS.Check(f.Ip, pattern)
	v := types.Bool(res)
	if err != nil {
		return types.ValOrErr(v, "%w: FCrDNS check failed", err)
	}
	return v
}
