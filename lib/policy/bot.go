package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/yl2chen/cidranger"
)

type Bot struct {
	Name      string
	UserAgent *regexp.Regexp
	Path      *regexp.Regexp
	Headers   map[string]*regexp.Regexp
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
	var headersRex string
	if len(b.Headers) > 0 {
		var sb strings.Builder
		sb.Grow(len(b.Headers) * 64)

		for name, expr := range b.Headers {
			sb.WriteString(name)
			sb.WriteString(expr.String())
		}

		headersRex = sb.String()
	}

	return internal.SHA256sum(fmt.Sprintf("%s::%s::%s::%s", b.Name, pathRex, userAgentRex, headersRex)), nil
}
