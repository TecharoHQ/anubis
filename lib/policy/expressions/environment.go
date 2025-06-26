package expressions

import (
	"math/rand/v2"
	"regexp"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	"github.com/google/cel-go/interpreter"
)

// BotEnvironment creates a new CEL environment, this is the set of
// variables and functions that are passed into the CEL scope so that
// Anubis can fail loudly and early when something is invalid instead
// of blowing up at runtime.
func BotEnvironment() (*cel.Env, error) {
	return New(
		cel.Types(FCrDNSType),
		// Variables exposed to CEL programs:
		cel.Variable("remoteAddress", cel.StringType),
		cel.Variable("host", cel.StringType),
		cel.Variable("method", cel.StringType),
		cel.Variable("userAgent", cel.StringType),
		cel.Variable("path", cel.StringType),
		cel.Variable("query", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("fcrdns", FCrDNSType),
		cel.Function("check", cel.MemberOverload("fcrdns_check_string", []*cel.Type{FCrDNSType, cel.StringType}, cel.BoolType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				f, ok := lhs.Value().(FCrDNS)
				if !ok {
					return types.ValOrErr(types.False, "receiver is not an fcrdns instance, but is %T", lhs)
				}
				pattern, ok := rhs.Value().(string)
				if !ok {
					return types.ValOrErr(types.False, "value is not a string, but is %T", rhs)
				}

				return f.check(pattern)
			}))),
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

var fcrdnsRegexOptimization = &interpreter.RegexOptimization{
	Function:   "check",
	RegexIndex: 1,
	Factory:    func(call interpreter.InterpretableCall, regexPattern string) (interpreter.InterpretableCall, error) {
		compiledRegex, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, err
		}

		return interpreter.NewCall(call.ID(), call.Function(), call.OverloadID(), call.Args(), func(values ...ref.Val) ref.Val {
			if len(values) != 2 {
				return types.NoSuchOverloadErr()
			}

			in, ok := values[0].Value().(FCrDNS)
			if !ok {
				return types.NoSuchOverloadErr()
			}
			return in.checkOptimized(compiledRegex)
		}), nil
	},
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
		// optimize regular expressions right now instead of on the fly
		cel.OptimizeRegex(interpreter.MatchesRegexOptimization, fcrdnsRegexOptimization),
		cel.EvalOptions(
			cel.OptOptimize,
		),
	)
}
