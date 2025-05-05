package policy

import (
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type CELChecker struct {
	src     string
	program cel.Program
}

func NewCELChecker(cfg *config.ExpressionOrList) (*CELChecker, error) {
	env, err := expressions.NewEnvironment()
	if err != nil {
		return nil, err
	}

	var src string
	var ast *cel.Ast

	if cfg.Expression != "" {
		src = cfg.Expression
		var iss *cel.Issues
		intermediate, iss := env.Compile(src)
		if iss != nil {
			return nil, iss.Err()
		}

		ast, iss = env.Check(intermediate)
		if iss != nil {
			return nil, iss.Err()
		}
	}

	if len(cfg.All) != 0 {
		ast, err = expressions.Join(env, expressions.JoinAnd, cfg.All...)
	}

	if len(cfg.Any) != 0 {
		ast, err = expressions.Join(env, expressions.JoinOr, cfg.Any...)
	}

	if err != nil {
		return nil, err
	}

	program, err := expressions.Compile(env, ast)
	if err != nil {
		return nil, fmt.Errorf("can't compile CEL program: %w", err)
	}

	return &CELChecker{
		src:     src,
		program: program,
	}, nil
}

func (cc *CELChecker) Hash() string {
	return internal.SHA256sum(cc.src)
}

func (cc *CELChecker) Check(r *http.Request) (bool, error) {
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
		return expressions.URLValues{Values: cr.URL.Query()}, true
	case "headers":
		return expressions.HTTPHeaders{Header: cr.Header}, true
	default:
		return nil, false
	}
}
