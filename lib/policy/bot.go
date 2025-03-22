package policy

import (
	"fmt"
	"regexp"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/yl2chen/cidranger"
)

type Bot struct {
	Name      string
	UserAgent *regexp.Regexp
	Path      *regexp.Regexp
	Action    config.Rule `json:"action"`
	Challenge *config.ChallengeRules
	Ranger    cidranger.Ranger
}

func (b Bot) Hash() (string, error) {
	var pathRex string
	if b.Path != nil {
		pathRex = b.Path.String()
	}
	var userAgentRex string
	if b.UserAgent != nil {
		userAgentRex = b.UserAgent.String()
	}

	return internal.SHA256sum(fmt.Sprintf("%s::%s::%s", b.Name, pathRex, userAgentRex)), nil
}
