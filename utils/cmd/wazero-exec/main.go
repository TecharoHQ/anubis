package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func defaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		path := filepath.Join(os.Getenv("HOME"), ".cache", "techaro.lol", "anubis", "wazero-exec")
		os.MkdirAll(path, 0755)
		return path
	}

	path := filepath.Join(dir, "techaro.lol", "anubis", "wazero-exec")
	os.MkdirAll(path, 0755)
	return path
}

var (
	cacheDir = flag.String("cache-dir", defaultCacheDir(), "default compilation cache folder")
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.wasm> [args]\n\n", filepath.Base(os.Args[0]))
		flag.Usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := run(ctx, flag.Arg(0), flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "can't run program %s: %v\n", flag.Arg(0), err)
	}
}

func run(ctx context.Context, fname string, args []string) error {
	cache, err := wazero.NewCompilationCacheWithDir(*cacheDir)
	if err != nil {
		return fmt.Errorf("can't open cache folder %q: %w", *cacheDir, err)
	}

	cfg := wazero.NewRuntimeConfig().
		WithCompilationCache(cache).
		WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling | experimental.CoreFeaturesExtendedConst | experimental.CoreFeaturesTailCall)

	runtime := wazero.NewRuntimeWithConfig(ctx, cfg)
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	bin, err := os.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("can't read file %s: %w", fname, err)
	}

	compiled, err := runtime.CompileModule(ctx, bin)
	if err != nil {
		return fmt.Errorf("can't compile binary %s: %w", fname, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("[unexpected] can't get cwd: %w", err)
	}

	fsConfig := wazero.NewFSConfig().WithDirMount(cwd, "/")

	config := wazero.NewModuleConfig().
		WithStdin(os.Stdin).WithStdout(os.Stdout).WithStderr(os.Stderr).
		WithArgs(args...).WithName(filepath.Base(fname)).WithFSConfig(fsConfig).
		WithSysNanosleep().WithSysNanotime().WithSysWalltime()

	for _, kp := range os.Environ() {
		kv := strings.SplitN(kp, "=", 2)
		config = config.WithEnv(kv[0], kv[1])
	}

	mod, err := runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return fmt.Errorf("can't run program %s: %w", fname, err)
	}
	defer mod.Close(ctx)

	return nil
}
