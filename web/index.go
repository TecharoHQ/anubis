package web

import (
	"github.com/a-h/templ"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/TecharoHQ/anubis/lib/policy/config"
)

func Base(title string, body templ.Component, impressum *config.Impressum, localizer *i18n.Localizer) templ.Component {
	return base(title, body, impressum, nil, nil, localizer)
}

func BaseWithChallengeAndOGTags(title string, body templ.Component, impressum *config.Impressum, challenge string, rules *config.ChallengeRules, ogTags map[string]string, localizer *i18n.Localizer) (templ.Component, error) {
	return base(title, body, impressum, struct {
		Rules     *config.ChallengeRules `json:"rules"`
		Challenge string                 `json:"challenge"`
	}{
		Challenge: challenge,
		Rules:     rules,
	}, ogTags, localizer), nil
}

func Index(localizer *i18n.Localizer) templ.Component {
	return index(localizer)
}

func ErrorPage(msg string, mail string, localizer *i18n.Localizer) templ.Component {
	return errorPage(msg, mail, localizer)
}

func Bench(localizer *i18n.Localizer) templ.Component {
	return bench(localizer)
}
