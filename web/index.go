package web

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
)

// Options carries per-server render state for Anubis pages. Embedders can
// render multiple Server instances in one process without mutating package
// globals.
type Options struct {
	BasePrefix string
	PublicURL  string
}

// DefaultOptions preserves the legacy package-global behavior for callers that
// render web components directly.
func DefaultOptions() Options {
	return Options{
		BasePrefix: anubis.BasePrefix,
		PublicURL:  anubis.PublicUrl,
	}
}

func Base(title string, body templ.Component, impressum *config.Impressum, localizer *localization.SimpleLocalizer) templ.Component {
	return BaseWithOptions(DefaultOptions(), title, body, impressum, localizer)
}

func BaseWithOptions(opts Options, title string, body templ.Component, impressum *config.Impressum, localizer *localization.SimpleLocalizer) templ.Component {
	return base(opts, title, body, impressum, nil, nil, localizer)
}

func BaseWithChallengeAndOGTags(title string, body templ.Component, impressum *config.Impressum, challenge *challenge.Challenge, rules *config.ChallengeRules, ogTags map[string]string, localizer *localization.SimpleLocalizer) templ.Component {
	return BaseWithChallengeAndOGTagsWithOptions(DefaultOptions(), title, body, impressum, challenge, rules, ogTags, localizer)
}

func BaseWithChallengeAndOGTagsWithOptions(opts Options, title string, body templ.Component, impressum *config.Impressum, challenge *challenge.Challenge, rules *config.ChallengeRules, ogTags map[string]string, localizer *localization.SimpleLocalizer) templ.Component {
	return base(opts, title, body, impressum, struct {
		Rules     *config.ChallengeRules `json:"rules"`
		Challenge any                    `json:"challenge"`
	}{
		Challenge: challenge,
		Rules:     rules,
	}, ogTags, localizer)
}

func ErrorPage(msg, mail, code string, localizer *localization.SimpleLocalizer) templ.Component {
	return ErrorPageWithOptions(DefaultOptions(), msg, mail, code, localizer)
}

func ErrorPageWithOptions(opts Options, msg, mail, code string, localizer *localization.SimpleLocalizer) templ.Component {
	return errorPage(opts, msg, mail, code, localizer)
}

func StaticHappy(localizer *localization.SimpleLocalizer) templ.Component {
	return StaticHappyWithOptions(DefaultOptions(), localizer)
}

func StaticHappyWithOptions(opts Options, localizer *localization.SimpleLocalizer) templ.Component {
	return staticHappy(opts, localizer)
}

func Bench(localizer *localization.SimpleLocalizer) templ.Component {
	return BenchWithOptions(DefaultOptions(), localizer)
}

func BenchWithOptions(opts Options, localizer *localization.SimpleLocalizer) templ.Component {
	return bench(opts, localizer)
}

func honeypotLink(href string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		fmt.Fprintf(w, `<script type="ignore"><a href="%s">Don't click me</a></script>`, href)
		return nil
	})
}
