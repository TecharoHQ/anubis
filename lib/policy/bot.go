package policy

import (
	"fmt"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/config"
)

type Bot struct {
	Name      string
	Action    config.Rule
	Challenge *config.ChallengeRules
	Rules     Checker
}

func (b Bot) Hash() string {
	return internal.SHA256sum(fmt.Sprintf("%s::%s", b.Name, b.Rules.Hash()))
}
