package policy

import (
	"fmt"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
)

type Bot struct {
	Rules     checker.Impl
	Challenge *config.ChallengeRules
	Weight    *config.Weight
	Name      string
	Action    config.Rule
}

func (b Bot) Hash() string {
	return internal.FastHash(fmt.Sprintf("%s::%s", b.Name, b.Rules.Hash()))
}
