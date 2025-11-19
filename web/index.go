package web

import (
	"github.com/a-h/templ"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
)

func Base(title string, body templ.Component, impressum *config.Impressum, localizer *localization.SimpleLocalizer) templ.Component {
	return base(title, body, impressum, nil, nil, localizer)
}

func BaseWithChallengeAndOGTags(title string, body templ.Component, impressum *config.Impressum, challenge *challenge.Challenge, rules *config.ChallengeRules, ogTags map[string]string, localizer *localization.SimpleLocalizer) templ.Component {
	return base(title, body, impressum, struct {
		Rules     *config.ChallengeRules `json:"rules"`
		Challenge any                    `json:"challenge"`
	}{
		Challenge: challenge,
		Rules:     rules,
	}, ogTags, localizer)
}

func ErrorPage(msg, mail, code string, localizer *localization.SimpleLocalizer) templ.Component {
	return errorPage(msg, mail, code, localizer)
}

func Bench(localizer *localization.SimpleLocalizer) templ.Component {
	return bench(localizer)
}
