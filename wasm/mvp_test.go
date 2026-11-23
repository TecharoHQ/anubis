package wasm

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/web"
	"github.com/tetratelabs/wazero"
)

func TestMVPArtifacts(t *testing.T) {
	ctx := t.Context()
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCoreFeatures(0))
	t.Cleanup(func() { _ = r.Close(ctx) })

	baseline, err := fs.Glob(web.Static, "static/wasm/baseline/*.wasm")
	if err != nil || len(baseline) == 0 {
		t.Fatalf("baseline artifacts missing: %v", err)
	}
	mvp, err := fs.Glob(web.Static, "static/wasm/mvp/*.wasm")
	if err != nil || len(mvp) != len(baseline) {
		t.Fatalf("MVP artifacts must match baseline artifacts: got %d, want %d: %v", len(mvp), len(baseline), err)
	}
	for i, baselinePath := range baseline {
		name := path.Base(baselinePath)
		if path.Base(mvp[i]) != name {
			t.Fatalf("MVP artifact %s does not match baseline artifact %s", mvp[i], baselinePath)
		}
		t.Run(name, func(t *testing.T) {
			data, err := web.Static.ReadFile("static/wasm/mvp/" + name)
			if err != nil {
				t.Fatal(err)
			}
			mod, err := r.CompileModule(ctx, data)
			if err != nil {
				t.Fatalf("artifact requires features beyond MVP: %v", err)
			}
			_ = mod.Close(ctx)
		})
	}
}

func TestMVPValidatorRejectsExtensions(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{
			name: "sign-extension",
			// (module (func (result i32) i32.const 255 i32.extend8_s))
			data: []byte{
				0, 0x61, 0x73, 0x6d, 1, 0, 0, 0,
				1, 5, 1, 0x60, 0, 1, 0x7f,
				3, 2, 1, 0,
				10, 8, 1, 6, 0, 0x41, 0xff, 1, 0xc0, 0x0b,
			},
		},
		{
			name: "bulk-memory",
			// A passive data segment requires DataCount and memory.init.
			data: []byte{
				0, 0x61, 0x73, 0x6d, 1, 0, 0, 0,
				1, 4, 1, 0x60, 0, 0,
				3, 2, 1, 0, 5, 3, 1, 0, 1,
				12, 1, 1,
				10, 14, 1, 12, 0, 0x41, 0, 0x41, 0, 0x41, 1, 0xfc, 8, 0, 0, 0x0b,
				11, 4, 1, 1, 1, 'x',
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			for _, strict := range []bool{false, true} {
				t.Run(fmt.Sprintf("strict=%t", strict), func(t *testing.T) {
					cfg := wazero.NewRuntimeConfig()
					if strict {
						cfg = cfg.WithCoreFeatures(0)
					}
					r := wazero.NewRuntimeWithConfig(ctx, cfg)
					t.Cleanup(func() { _ = r.Close(ctx) })
					_, err := r.CompileModule(ctx, tc.data)
					if (err != nil) != strict {
						t.Fatalf("strict=%t: unexpected validation result: %v", strict, err)
					}
				})
			}
		})
	}
}

func TestMVPProofCompatibility(t *testing.T) {
	for _, name := range []string{"argon2id", "hashx", "sha256"} {
		t.Run(name, func(t *testing.T) {
			if testing.Short() && name == "argon2id" && runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
				t.Skipf("skipping argon2id on %s in short mode because wazero has no compiler for it and the interpreter is too slow", runtime.GOARCH)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			t.Cleanup(cancel)
			runners := make([]*Runner, 0, 2)
			for _, tier := range []string{"baseline", "mvp"} {
				fin, err := web.Static.Open("static/wasm/" + tier + "/" + name + ".wasm")
				if err != nil {
					t.Fatal(err)
				}
				runner, err := NewRunner(ctx, name, fin)
				_ = fin.Close()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = runner.r.Close(ctx) })
				runners = append(runners, runner)
			}
			for _, last := range []byte{0, 255} {
				for _, initialNonce := range []uint32{0, 123} {
					t.Run(fmt.Sprintf("last=%d/nonce=%d", last, initialNonce), func(t *testing.T) {
						data := bytes.Repeat([]byte{42}, 32)
						data[len(data)-1] = last
						var expectedNonce uint32
						var expectedHash []byte
						for i, runner := range runners {
							nonce, hash, err := runner.Run(ctx, data, 1, initialNonce, 1)
							if err != nil {
								t.Fatal(err)
							}
							if i == 0 {
								expectedNonce, expectedHash = nonce, bytes.Clone(hash)
							} else if nonce != expectedNonce || !bytes.Equal(hash, expectedHash) {
								t.Fatalf("MVP proof differs from baseline: nonce=%d, want %d; hash=%x, want %x", nonce, expectedNonce, hash, expectedHash)
							}
						}
						for _, runner := range runners {
							ok, err := runner.Verify(ctx, data, expectedHash, expectedNonce, 1)
							if err != nil || !ok {
								t.Fatalf("valid proof rejected: ok=%t: %v", ok, err)
							}
							corrupt := bytes.Clone(expectedHash)
							corrupt[0] ^= 1
							ok, err = runner.Verify(ctx, data, corrupt, expectedNonce, 1)
							if err != nil || ok {
								t.Fatalf("corrupt proof accepted or failed to execute: ok=%t: %v", ok, err)
							}
						}
					})
				}
			}
		})
	}
}
