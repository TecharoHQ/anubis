package challengepage

import (
	"html"
	"regexp"
)

//go:generate go tool github.com/a-h/templ/cmd/templ generate

// wikipediaLink matches an escaped Wikipedia anchor tag in translated text.
// The URL character set is restricted so that nothing in it can break out of
// the href attribute.
var wikipediaLink = regexp.MustCompile(`&lt;a href=&#34;(https://[a-z-]{2,12}\.wikipedia\.org/wiki/[A-Za-z0-9_-]+)&#34;&gt;(.*?)&lt;/a&gt;`)

// explanationHTML escapes translated text and then restores only anchor tags
// pointing at Wikipedia. Translations are community-contributed, so they must
// never be rendered as raw HTML.
func explanationHTML(text string) string {
	return wikipediaLink.ReplaceAllString(html.EscapeString(text), `<a href="$1">$2</a>`)
}
