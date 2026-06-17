//go:build arm64

package wasm

import "github.com/tetratelabs/wazero"

// runtimeConfig selects the wazero runtime configuration.
//
// On arm64 the wazero optimizing compiler miscompiles the HashX proof-of-work
// module such that anubis_work never terminates (see the "never terminates on
// arm64" regression). The interpreter executes the same module correctly, so we
// trade raw throughput for correctness on this architecture.
func runtimeConfig() wazero.RuntimeConfig {
	return wazero.NewRuntimeConfigInterpreter()
}
