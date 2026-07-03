package internal

import "mime"

func init() {
	_ = mime.AddExtensionType(".mjs", "text/javascript")
}
