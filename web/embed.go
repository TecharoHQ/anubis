package web

import "embed"

var (
	//go:embed static
	Static embed.FS
)
