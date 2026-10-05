package policy

import (
	"fmt"
	"net/http"
	"net/url"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dns"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
)

type CELChecker struct {
	program        cel.Program
	src            string
	subRequestMode bool
}

func NewCELChecker(cfg *config.ExpressionOrList, dnsObj *dns.Dns, subRequestMode bool) (*CELChecker, error) {
	env, err := expressions.BotEnvironment(dnsObj)
	if err != nil {
		return nil, err
	}

	program, err := expressions.Compile(env, cfg.String())
	if err != nil {
		return nil, fmt.Errorf("can't compile CEL program: %w", err)
	}

	return &CELChecker{
		src:            cfg.String(),
		program:        program,
		subRequestMode: subRequestMode,
	}, nil
}

func (cc *CELChecker) Hash() string {
	return internal.FastHash(cc.src)
}

func (cc *CELChecker) Check(r *http.Request) (bool, error) {
	result, _, err := cc.program.ContextEval(r.Context(), &CELRequest{r, cc.subRequestMode})

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
	subRequestMode bool
}

func (cr *CELRequest) Parent() cel.Activation { return nil }

func (cr *CELRequest) ResolveName(name string) (any, bool) {
	switch name {
	case "remoteAddress":
		return cr.Header.Get("X-Real-IP"), true
	case "contentLength":
		return cr.ContentLength, true
	case "host":
		return cr.Host, true
	case "method":
		return cr.Method, true
	case "userAgent":
		return cr.UserAgent(), true
	case "path":
		if cr.subRequestMode {
			original := cr.Header.Get("X-Original-Uri")
			if original == "" {
				original = cr.Header.Get("X-Forwarded-Uri")
			}
			if original != "" {
				u, err := url.ParseRequestURI(original)
				if err != nil {
					return nil, false
				}
				return pathForPolicy(u.Path), true
			}
		}
		return pathForPolicy(cr.URL.Path), true
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
		return expressions.ResolveBotVariable(name, cr.Request)
	}
}
