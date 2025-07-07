// Package all is a meta-package that imports all store implementations.
//
// This is a HACK to make tests work consistently.
package all

import (
	_ "github.com/TecharoHQ/anubis/lib/store/bbolt"
	_ "github.com/TecharoHQ/anubis/lib/store/memory"
	_ "github.com/TecharoHQ/anubis/lib/store/valkey"
)
