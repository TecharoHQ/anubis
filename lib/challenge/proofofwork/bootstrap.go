package proofofwork

import (
	"context"
	_ "embed"
	"io"

	"github.com/a-h/templ"
)

//go:embed static/js/bootstrap.mjs
var scriptBytes string

func bootstrap() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, "<script>"+scriptBytes+"</script>"); err != nil {
			return err
		}
		return nil
	})
}
