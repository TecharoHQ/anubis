package policy

import (
	"log/slog"

	"github.com/TecharoHQ/anubis/lib/config"
)

type CheckResult struct {
	Name   string
	Rule   config.Rule
	Weight int
}

func (cr CheckResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", cr.Name),
		slog.String("rule", string(cr.Rule)),
		slog.Int("weight", cr.Weight),
	)
}
