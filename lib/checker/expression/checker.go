package expression

import (
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker/expression/environment"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type Checker struct {
	program cel.Program
	src     string
	hash    string
}

func New(cfg *Config) (*Checker, error) {
	env, err := environment.Bot()
	if err != nil {
		return nil, err
	}

	program, err := environment.Compile(env, cfg.String())
	if err != nil {
		return nil, fmt.Errorf("can't compile CEL program: %w", err)
	}

	return &Checker{
		src:     cfg.String(),
		hash:    internal.FastHash(cfg.String()),
		program: program,
	}, nil
}

func (cc *Checker) Hash() string {
	return cc.hash
}

func (cc *Checker) Check(r *http.Request) (bool, error) {
	result, _, err := cc.program.ContextEval(r.Context(), &CELRequest{r})

	if err != nil {
		return false, err
	}

	if val, ok := result.(types.Bool); ok {
		return bool(val), nil
	}

	return false, nil
}

type CELRequest struct {
	*http.Request
}

func (cr *CELRequest) Parent() cel.Activation { return nil }

func (cr *CELRequest) ResolveName(name string) (any, bool) {
	switch name {
	case "remoteAddress":
		return cr.Header.Get("X-Real-Ip"), true
	case "host":
		return cr.Host, true
	case "method":
		return cr.Method, true
	case "userAgent":
		return cr.UserAgent(), true
	case "path":
		return cr.URL.Path, true
	case "query":
		return URLValues{Values: cr.URL.Query()}, true
	case "headers":
		return HTTPHeaders{Header: cr.Header}, true
	case "load_1m":
		return Load1(), true
	case "load_5m":
		return Load5(), true
	case "load_15m":
		return Load15(), true
	default:
		return nil, false
	}
}
