package expressions

import (
	"math/rand/v2"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"github.com/google/cel-go/ext"
)

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
		cel.Variable("load_1m", cel.DoubleType),
		cel.Variable("load_5m", cel.DoubleType),
		cel.Variable("load_15m", cel.DoubleType),

		// Bot-specific functions:
		cel.Function("missingHeader",
			cel.Overload("missingHeader_map_string_string_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(headers, key ref.Val) ref.Val {
					// Convert headers to a trait that supports Find
					headersMap, ok := headers.(traits.Indexer)
					if !ok {
						return types.ValOrErr(headers, "headers is not a map, but is %T", headers)
					}

					keyStr, ok := key.(types.String)
					if !ok {
						return types.ValOrErr(key, "key is not a string, but is %T", key)
					}

					val := headersMap.Get(keyStr)
					// Check if the key is missing by testing for an error
					if types.IsError(val) {
						return types.Bool(true) // header is missing
					}
					return types.Bool(false) // header is present
				}),
			),
		),
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
