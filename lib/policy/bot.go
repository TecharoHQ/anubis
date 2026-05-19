package policy

import (
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
)

type Bot struct {
	Rules     checker.Impl
	Challenge *config.ChallengeRules
	Weight    *config.Weight
	Name      string
	// hash caches the result of Hash() when populated at parse time, see ParseConfig
	hash   string
	Action config.Rule
}

// Hash returns a stable identifier for this Bot derived from its Name
// and Rules. When the cached value is present (populated by
// ParseConfig) it is returned directly; otherwise the hash is
// recomputed on demand so callers do not have to know about the cache.
func (b Bot) Hash() string {
	if b.hash != "" {
		return b.hash
	}
	var rulesHash string
	if b.Rules != nil { // defensive, should never happen
		rulesHash = b.Rules.Hash()
	}
	return internal.FastHash(b.Name + "::" + rulesHash)
}
