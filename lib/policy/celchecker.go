package policy

import (
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type CELChecker struct {
	program cel.Program
	src     string
}

func NewCELChecker(cfg *config.ExpressionOrList, dnsObj *internal.Dns) (*CELChecker, error) {
	env, err := expressions.BotEnvironment(dnsObj)
	if err != nil {
		return nil, err
	}

	program, err := expressions.Compile(env, cfg.String())
	if err != nil {
		return nil, fmt.Errorf("can't compile CEL program: %w", err)
	}

	return &CELChecker{
		src:     cfg.String(),
		program: program,
	}, nil
}

func (cc *CELChecker) Hash() string {
	return internal.FastHash(cc.src)
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
	case "contentLength":
		return cr.ContentLength, true
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
	case "load_1m":
		return expressions.Load1(), true
	case "load_5m":
		return expressions.Load5(), true
	case "load_15m":
		return expressions.Load15(), true
	default:
		return nil, false
	}
}
