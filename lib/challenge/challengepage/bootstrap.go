// Package challengepage holds the parts of the challenge page that every
// JavaScript-driven challenge method needs.
package challengepage

import (
	"context"
	_ "embed"
	"io"

	"github.com/a-h/templ"
)

//go:generate ./build.sh

//go:embed static/js/bootstrap.mjs
var scriptBytes string

// Bootstrap renders the inline watchdog script that re-injects main.mjs when
// it fails to load. It must be rendered after the script tag it watches so
// that the tag exists by the time this runs during page parse.
func Bootstrap() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<script>"+scriptBytes+"</script>"); err != nil {
			return err
		}
		return nil
	})
}
