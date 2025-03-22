package lib

import (
	"log/slog"

	"github.com/TecharoHQ/anubis/lib/policy/config"
)

type CheckResult struct {
	Name string
	Rule config.Rule
}

func (cr CheckResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", cr.Name),
		slog.String("rule", string(cr.Rule)))
}

func cr(name string, rule config.Rule) CheckResult {
	return CheckResult{
		Name: name,
		Rule: rule,
	}
}
