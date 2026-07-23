package web

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
)

func Base(title string, body templ.Component, impressum *config.Impressum, honeypot *config.Honeypot, localizer *localization.SimpleLocalizer) templ.Component {
	return base(title, body, impressum, honeypot, nil, nil, localizer)
}

func BaseWithChallengeAndOGTags(title string, body templ.Component, impressum *config.Impressum, honeypot *config.Honeypot, challenge *challenge.Challenge, rules *config.ChallengeRules, ogTags map[string]string, localizer *localization.SimpleLocalizer) templ.Component {
	return base(title, body, impressum, honeypot, struct {
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

func honeypotLink(href string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := fmt.Fprintf(w, `<script type="ignore"><a href="%s">Don't click me</a></script>`, href); err != nil {
			return err
		}
		return nil
	})
}
