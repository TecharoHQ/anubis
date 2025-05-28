package internal

import "mime"

func init() {
	mime.AddExtensionType(".mjs", "text/javascript")
}
