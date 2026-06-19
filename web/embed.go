package web

import "embed"

//go:generate go tool github.com/a-h/templ/cmd/templ generate --log-level error

var (
	//go:embed static
	Static embed.FS
)
