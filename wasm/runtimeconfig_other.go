//go:build !arm64

package wasm

import "github.com/tetratelabs/wazero"

// runtimeConfig selects the wazero runtime configuration. Everywhere except
// arm64 the default (optimizing compiler where available) is correct and fast.
func runtimeConfig() wazero.RuntimeConfig {
	return wazero.NewRuntimeConfig()
}
