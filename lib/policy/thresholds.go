package policy

import (
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/lib/policy/expressions"
	"github.com/google/cel-go/cel"
)

type Threshold struct {
	config.Threshold
	program cel.Program
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

	result.program = program

	return result, nil
}
