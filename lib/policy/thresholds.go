package policy

import (
	"cel.dev/cel-go/cel"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
)

type Threshold struct {
	config.Threshold
	Program cel.Program
}

func ParsedThresholdFromConfig(t config.Threshold) (*Threshold, error) {
	result := &Threshold{
		Threshold: t,
	}

	env, err := expressions.ThresholdEnvironment()
	if err != nil {
		return nil, err
	}

	program, err := expressions.Compile(env, t.Expression.String())
	if err != nil {
		return nil, err
	}

	result.Program = program

	return result, nil
}

type ThresholdRequest struct {
	Weight int
}

func (tr *ThresholdRequest) Parent() cel.Activation { return nil }

func (tr *ThresholdRequest) ResolveName(name string) (any, bool) {
	switch name {
	case "weight":
		return tr.Weight, true
	default:
		return nil, false
	}
}
