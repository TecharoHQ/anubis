package expressions

import (
	"fmt"
	"math/rand/v2"
	"net"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
	"github.com/yl2chen/cidranger"
)

// pre-parsed CIDR ranger map. Hash of CIDR IP list is key
var CIDRMap = make(map[string]cidranger.Ranger)

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
	return internal.SHA256sum(strings.Join(cidrs, "|"))
}

// getCachedRanger returns a cached ranger or builds a new one
func getCachedRanger(ipList traits.Lister) (cidranger.Ranger, error) {
	// Build cache key
	cacheKey := buildCacheKey(ipList)

	// Check cache
	if ranger, ok := CIDRMap[cacheKey]; ok {
		return ranger, nil
	}

	// Build new ranger
	ranger := cidranger.NewPCTrieRanger()

	it := ipList.Iterator()
	for it.HasNext() == types.True {
		item := it.Next()
		cidr, ok := item.(types.String)
		if !ok {
			continue
		}
		_, rng, err := net.ParseCIDR(string(cidr))
		if err != nil {
			return nil, fmt.Errorf("address %s CIDR parse error: %w", cidr, err)
		}
		ranger.Insert(cidranger.NewBasicRangerEntry(*rng))
	}

	// Store in map
	CIDRMap[cacheKey] = ranger
	return ranger, nil
}

func remoteAddrInList(remoteAddr types.String, ipList traits.Lister) (bool, error) {
	ipAddr := net.ParseIP(string(remoteAddr))
	if ipAddr == nil {
		return false, fmt.Errorf("remoteAddrInList: %s is not a valid IP address", remoteAddr)
	}

	ranger, err := getCachedRanger(ipList)
	if err != nil {
		return false, fmt.Errorf("remoteAddrInList: %v", err)
	}

	ok, err := ranger.Contains(ipAddr)
	if err != nil {
		return false, fmt.Errorf("remoteAddrInList: error checking if %s is in range: %v", remoteAddr, err)
	}
	return ok, nil
}

// NewEnvironment creates a new CEL environment, this is the set of
// variables and functions that are passed into the CEL scope so that
// Anubis can fail loudly and early when something is invalid instead
// of blowing up at runtime.
func NewEnvironment() (*cel.Env, error) {
	return cel.NewEnv(
		ext.Strings(
			ext.StringsLocale("en_US"),
			ext.StringsValidateFormatCalls(true),
		),
		// default all timestamps to UTC
		cel.DefaultUTCTimeZone(true),
		// Variables exposed to CEL programs:
		cel.Variable("remoteAddress", cel.StringType),
		cel.Variable("host", cel.StringType),
		cel.Variable("method", cel.StringType),
		cel.Variable("userAgent", cel.StringType),
		cel.Variable("path", cel.StringType),
		cel.Variable("query", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		// Functions exposed to CEL programs:
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
	)
}

// Compile takes CEL environment and syntax tree then emits an optimized
// Program for execution.
func Compile(env *cel.Env, ast *cel.Ast) (cel.Program, error) {
	return env.Program(
		ast,
		cel.EvalOptions(
			// optimize regular expressions right now instead of on the fly
			cel.OptOptimize,
		),
	)
}
