package expressions

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/gaissmai/bart"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
)

// pre-parsed CIDR bart tables map. Hash of CIDR IP list is key
var CIDRMap = make(map[string]*bart.Lite)

// buildCacheKey creates a deterministic cache key from the IP list
func buildCacheKey(ipList traits.Lister) string {
	var cidrs []string
	it := ipList.Iterator()
	for it.HasNext() == types.True {
		item := it.Next()
		cidr, ok := item.(types.String)
		if !ok {
			continue
		}
		cidrs = append(cidrs, string(cidr))
	}
	// Join them to create a unique key
	return internal.FastHash(strings.Join(cidrs, "|"))
}

// getCachedPrefixTable returns a cached bart table or builds a new one
func getCachedPrefixTable(ipList traits.Lister) (*bart.Lite, error) {
	// Build cache key
	cacheKey := buildCacheKey(ipList)

	// Check cache
	if prefixtable, ok := CIDRMap[cacheKey]; ok {
		return prefixtable, nil
	}

	// Build new bart table
	prefixtable := new(bart.Lite)

	it := ipList.Iterator()
	for it.HasNext() == types.True {
		item := it.Next()
		cidr, ok := item.(types.String)
		if !ok {
			continue
		}
		prefix, err := netip.ParsePrefix(string(cidr))
		if err != nil {
			return nil, fmt.Errorf("address %s CIDR parse error: %w", cidr, err)
		}
		prefixtable.Insert(prefix)
	}

	// Store in map
	CIDRMap[cacheKey] = prefixtable
	return prefixtable, nil
}

func remoteAddrInList(remoteAddr types.String, ipList traits.Lister) (bool, error) {
	ipAddr, err := netip.ParseAddr(string(remoteAddr))
	if err != nil {
		return false, fmt.Errorf("remoteAddrInList: %s is not a valid IP address", remoteAddr)
	}

	prefixtable, err := getCachedPrefixTable(ipList)
	if err != nil {
		return false, fmt.Errorf("remoteAddrInList: %v", err)
	}

	ok := prefixtable.Contains(ipAddr)
	return ok, nil
}

// BotEnvironment creates a new CEL environment, this is the set of
// variables and functions that are passed into the CEL scope so that
// Anubis can fail loudly and early when something is invalid instead
// of blowing up at runtime.
func BotEnvironment() (*cel.Env, error) {
	return New(
		// Variables exposed to CEL programs:
		cel.Variable("remoteAddress", cel.StringType),
		cel.Variable("host", cel.StringType),
		cel.Variable("method", cel.StringType),
		cel.Variable("userAgent", cel.StringType),
		cel.Variable("path", cel.StringType),
		cel.Variable("query", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
	)
}

// NewThreshold creates a new CEL environment for threshold checking.
func ThresholdEnvironment() (*cel.Env, error) {
	return New(
		cel.Variable("weight", cel.IntType),
	)
}

func New(opts ...cel.EnvOption) (*cel.Env, error) {
	args := []cel.EnvOption{
		ext.Strings(
			ext.StringsLocale("en_US"),
			ext.StringsValidateFormatCalls(true),
		),

		// default all timestamps to UTC
		cel.DefaultUTCTimeZone(true),

		// Functions exposed to all CEL programs:
		cel.Function("randInt",
			cel.Overload("randInt_int",
				[]*cel.Type{cel.IntType},
				cel.IntType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					n, ok := val.(types.Int)
					if !ok {
						return types.ValOrErr(val, "value is not an integer, but is %T", val)
					}

					return types.Int(rand.IntN(int(n)))
				}),
			),
		),
		cel.Function("remoteAddrInList",
			cel.Overload(
				"remoteAddrInList_bool",
				[]*cel.Type{cel.StringType, cel.ListType(cel.StringType)},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					val, err := remoteAddrInList(args[0].(types.String), args[1].(traits.Lister))
					if err != nil {
						// CEL expects errors to be returned as types.Err
						return types.NewErr("%s", err.Error())
					}

					return types.Bool(val)
				}),
			),
		),
	}

	args = append(args, opts...)
	return cel.NewEnv(args...)
}

// Compile takes CEL environment and syntax tree then emits an optimized
// Program for execution.
func Compile(env *cel.Env, src string) (cel.Program, error) {
	intermediate, iss := env.Compile(src)
	if iss != nil {
		return nil, iss.Err()
	}

	ast, iss := env.Check(intermediate)
	if iss != nil {
		return nil, iss.Err()
	}

	return env.Program(
		ast,
		cel.EvalOptions(
			// optimize regular expressions right now instead of on the fly
			cel.OptOptimize,
		),
	)
}
