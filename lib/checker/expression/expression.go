package expression

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dns"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

var (
	ErrInvalidConfig = errors.New("checker.expression: invalid config")
	ErrCantCompile   = errors.New("checker.expression: can't compile CEL program")
)

func init() {
	checker.Register("expression", Factory{})
}

// Config describes a CEL expression check. It reuses [config.ExpressionOrList],
// which accepts either a bare expression string or an object with an "all" or
// "any" list. The list forms are flattened into a single CEL program by joining
// the predicates with && (all) or || (any); see [config.ExpressionOrList.String].
type Config = config.ExpressionOrList

type Factory struct{}

func (Factory) ValidateConfig(ctx context.Context, inp json.RawMessage) error {
	var c Config
	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := c.Valid(); err != nil {
		return errors.Join(ErrInvalidConfig, err)
	}

	// A DNS resolver may not be available at validation time; that's fine, the
	// program only needs one when it actually evaluates DNS functions.
	d, _ := dns.FromContext(ctx)
	if _, err := compile(d, c.String()); err != nil {
		return err
	}

	return nil
}

func (Factory) Create(ctx context.Context, inp json.RawMessage) (checker.Impl, error) {
	var c Config
	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := c.Valid(); err != nil {
		return nil, errors.Join(ErrInvalidConfig, err)
	}

	d, _ := dns.FromContext(ctx)
	program, err := compile(d, c.String())
	if err != nil {
		return nil, err
	}

	return &Impl{
		src:        c.String(),
		program:    program,
		subrequest: checker.SubrequestMode(ctx),
		hash:       internal.SHA256sum(string(inp)),
	}, nil
}

func compile(d *dns.Dns, src string) (cel.Program, error) {
	env, err := expressions.BotEnvironment(d)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantCompile, err)
	}

	program, err := expressions.Compile(env, src)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCantCompile, err)
	}

	return program, nil
}

type Impl struct {
	program    cel.Program
	src        string
	subrequest bool
	hash       string
}

func (i *Impl) Check(r *http.Request) (bool, error) {
	result, _, err := i.program.ContextEval(r.Context(), &request{Request: r, subrequest: i.subrequest})
	if err != nil {
		return false, err
	}

	if val, ok := result.(types.Bool); ok {
		return bool(val), nil
	}

	return false, nil
}

func (i *Impl) Hash() string { return i.hash }

// request adapts an *http.Request into a cel.Activation, exposing the request's
// salient fields to CEL programs.
type request struct {
	*http.Request
	subrequest bool
}

func (cr *request) Parent() cel.Activation { return nil }

func (cr *request) ResolveName(name string) (any, bool) {
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
		if cr.subrequest {
			if xou := cr.Header.Get("X-Original-URI"); xou != "" {
				return xou, true
			}
			if xfu := cr.Header.Get("X-Forwarded-Uri"); xfu != "" {
				return xfu, true
			}
		}
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
