package wasm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWASMBuildRecovery(t *testing.T) {
	root := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		name = filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"build_wasm.sh", "lib.sh"} {
		data, err := os.ReadFile(filepath.Join("scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		write("wasm/scripts/"+name, string(data))
	}
	for _, name := range []string{"argon2id", "hashx", "sha256"} {
		write("wasm/pow/"+name+"/Cargo.toml", "")
	}
	for _, name := range []string{"Cargo.toml", "Cargo.lock", "wasm/anubis/src/lib.rs", "utils/wasm/wasm2js/wasm-opt_130.wasm"} {
		write(name, "")
	}
	write("bin/cargo", `#!/usr/bin/env bash
set -eu
if [ "$1" = build ]; then
  mkdir -p target/wasm32-unknown-unknown/release
  for algorithm in argon2id hashx sha256; do
    echo wasm > "target/wasm32-unknown-unknown/release/${algorithm}.wasm"
  done
fi
`)
	write("bin/wasm-opt", `#!/usr/bin/env bash
set -eu
input= output= previous=
for arg in "$@"; do
  if [ "$arg" = --llvm-memory-copy-fill-lowering ] && [ "${FAIL_MVP:-}" = 1 ]; then
    exit 9
  fi
  if [ "$previous" = -o ]; then output="$arg";
  elif [[ "$arg" == *.wasm ]]; then input="$arg"; fi
  previous="$arg"
done
cp "$input" "$output"
`)
	run := func(fail bool) (string, error) {
		t.Helper()
		cmd := exec.Command("bash", "wasm/scripts/build_wasm.sh")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
		if fail {
			cmd.Env = append(cmd.Env, "FAIL_MVP=1")
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	for attempt := 0; attempt < 2; attempt++ {
		out, err := run(true)
		if err == nil || strings.Contains(out, "up to date") {
			t.Fatalf("attempt %d skipped an unfinished MVP build: err=%v\n%s", attempt, err, out)
		}
	}
	if out, err := run(false); err != nil || strings.Contains(out, "up to date") {
		t.Fatalf("failed to recover the build: %v\n%s", err, out)
	}
	if out, err := run(false); err != nil || !strings.Contains(out, "up to date") {
		t.Fatalf("completed build was not cached: %v\n%s", err, out)
	}
	if err := os.Remove(filepath.Join(root, "web/static/wasm/mvp/hashx.wasm")); err != nil {
		t.Fatal(err)
	}
	if out, err := run(false); err != nil || strings.Contains(out, "up to date") {
		t.Fatalf("partial artifact set was cached: %v\n%s", err, out)
	}
	script := filepath.Join(root, "wasm/scripts/build_wasm.sh")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(script, future, future); err != nil {
		t.Fatal(err)
	}
	if out, err := run(false); err != nil || strings.Contains(out, "up to date") {
		t.Fatalf("build script changes did not invalidate artifacts: %v\n%s", err, out)
	}
}
