package expressions

import (
	"log"
	"net/http"

	"github.com/google/cel-go/cel"
)

// BotVariableResolver returns the value of a registered bot variable for
// a request. This is how extensions can add additional variables to bot
// contexts.
type BotVariableResolver func(r *http.Request) (any, bool)

type botVariable struct {
	name    string
	typ     *cel.Type
	resolve BotVariableResolver
}

var (
	botVariables = map[string]botVariable{}

	// builtinBotVariables is the list of variables that the CEL environment
	// already defines. This is here so that extensions can't add a variable
	// that already exists.
	builtinBotVariables = map[string]struct{}{
		"remoteAddress": {},
		"contentLength": {},
		"host":          {},
		"method":        {},
		"userAgent":     {},
		"path":          {},
		"query":         {},
		"headers":       {},
		"load_1m":       {},
		"load_5m":       {},
		"load_15m":      {},
	}
)

// RegisterBotVariable adds a variable to the bot rule environment.
//
// This function assumes it's being called from an init() hook at the beginning
// of program execution. Any attempt to call it after Anubis has started can
// and will cause undefined behaviour up to and including a crash.
//
// If the bot variable being registered conflicts with a built-in bot variable,
// this function will panic and take out the program early in its lifecycle. The
// hope is that this will fire when tests are run, which should prevent errant
// unfeatures from being shipped.
func RegisterBotVariable(name string, typ *cel.Type, resolve BotVariableResolver) {
	if _, ok := builtinBotVariables[name]; ok {
		log.Panicf("cannot register bot variable %q, it's built in", name)
	}

	if _, ok := botVariables[name]; ok {
		log.Panicf("cannot register bot variable %q, it's already registered", name)
	}

	botVariables[name] = botVariable{name: name, typ: typ, resolve: resolve}
}

// ResolveBotVariable resolves a bot variable that was registered with
// [RegisterBotVariable], calling the arbitrary [BotVariableResolver] and
// returning the values to the caller.
func ResolveBotVariable(name string, r *http.Request) (any, bool) {
	v, ok := botVariables[name]
	if !ok {
		return nil, false
	}

	return v.resolve(r)
}

// getBotVariables returns the bot variables as a []cel.EnvOption for building
// CEL checkers.
func getBotVariables() []cel.EnvOption {
	result := make([]cel.EnvOption, 0, len(botVariables))

	for _, v := range botVariables {
		result = append(result, cel.Variable(v.name, v.typ))
	}

	return result
}
