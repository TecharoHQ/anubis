package web

import "embed"

//go:generate go tool github.com/a-h/templ/cmd/templ generate
//go:generate go tool gosh ./build.sh

var (
	//go:embed static
	Static embed.FS
)
