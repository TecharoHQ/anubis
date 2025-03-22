package web

import "embed"

//go:generate go tool github.com/a-h/templ/cmd/templ generate
//go:generate esbuild js/main.mjs --sourcemap --bundle --minify --outfile=static/js/main.mjs
//go:generate gzip -f -k static/js/main.mjs
//go:generate zstd -f -k --ultra -22 static/js/main.mjs
//go:generate brotli -fZk static/js/main.mjs

var (
	//go:embed static
	Static embed.FS
)
