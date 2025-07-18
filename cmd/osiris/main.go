package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/cmd/osiris/internal/entrypoint"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/facebookgo/flagenv"
)

var (
	configFname = flag.String("config", "./osiris.hcl", "Configuration file (HCL), see docs")
	slogLevel   = flag.String("slog-level", "INFO", "logging level (see https://pkg.go.dev/log/slog#hdr-Levels)")
	versionFlag = flag.Bool("version", false, "if true, show version information then quit")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	if *versionFlag {
		fmt.Println("Osiris", anubis.Version)
		return
	}

	internal.InitSlog(*slogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := entrypoint.Main(ctx, entrypoint.Options{
		ConfigFname: *configFname,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
