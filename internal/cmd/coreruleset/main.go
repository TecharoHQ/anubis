package main

import (
	"flag"
	"log"
	"os"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/maybedoer"
)

var (
	outDir    = flag.String("out-dir", "./data/security/coreruleset", "output folder for coreruleset rules")
	slogLevel = flag.String("slog-level", "info", "logging level")
)

func main() {
	flag.Parse()

	internal.InitSlog(*slogLevel, os.Stderr)

	c := new(maybedoer.Chain)

	c.Maybe(aiCriticalArtifacts)
	c.Maybe(lfiOsFiles)
	c.Maybe(restrictedFiles)

	if err := c.Error(); err != nil {
		log.Fatal(err)
	}
}
