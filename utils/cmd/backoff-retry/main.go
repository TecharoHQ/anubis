package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	startWait = flag.Duration("start-wait", 250*time.Millisecond, "amount of time to start with exponential backoff")
	tryCount  = flag.Int("try-count", 5, "number of retries")
)

func main() {
	flag.Parse()

	cmdStr := strings.Join(flag.Args(), " ")
	wait := *startWait

	for i := range make([]struct{}, *tryCount) {
		slog.Info("executing", "try", i+1, "wait", wait, "cmd", cmdStr)

		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Stdin = nil
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			time.Sleep(wait)
			wait = wait * 2
		} else {
			os.Exit(0)
		}
	}

	fmt.Printf("giving up after %d tries\n", *tryCount)
	os.Exit(1)
}
