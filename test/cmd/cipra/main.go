package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/test/cmd/cipra/internal"
	"github.com/facebookgo/flagenv"
)

var (
	bind                 = flag.String("bind", ":9090", "TCP host:port to bind HTTP on")
	browserBin           = flag.String("browser-bin", "palemoon", "browser binary name")
	browserContainerName = flag.String("browser-container-name", "palemoon", "browser container name")
	composeName          = flag.String("compose-name", "", "docker compose base name for resources")
	vncServerContainer   = flag.String("vnc-container-name", "display", "VNC host:port (NOT a display number)")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	lanip, err := internal.GetLANIP()
	if err != nil {
		log.Panic(err)
	}

	os.Setenv("TARGET", fmt.Sprintf("%s%s", lanip.String(), *bind))

	http.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OK", http.StatusOK)
		log.Println("got termination signal", r.RequestURI)
		go func() {
			time.Sleep(2 * time.Second)
			cancel()
		}()
	})

	srv := &http.Server{
		Handler: http.DefaultServeMux,
		Addr:    *bind,
	}

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Panic(err)
		}
	}()

	if err := RunScript(ctx, "docker", "compose", "up", "-d"); err != nil {
		log.Fatalf("can't start project: %v", err)
	}

	defer RunScript(ctx, "docker", "compose", "down", "-t", "1")
	defer RunScript(ctx, "docker", "compose", "rm", "-f")

	internal.UnbreakDocker(*composeName + "_default")

	if err := RunScript(ctx, "docker", "exec", fmt.Sprintf("%s-%s-1", *composeName, *browserContainerName), "bash", "/hack/scripts/install-cert.sh"); err != nil {
		log.Panic(err)
	}

	if err := RunScript(ctx, "docker", "exec", fmt.Sprintf("%s-%s-1", *composeName, *browserContainerName), *browserBin, "https://relayd"); err != nil {
		log.Panic(err)
	}

	<-ctx.Done()
	srv.Close()
	time.Sleep(2 * time.Second)
}

func RunScript(ctx context.Context, args ...string) error {
	var err error
	backoff := 250 * time.Millisecond

	for attempt := 0; attempt < 5; attempt++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		log.Printf("Running command: %s", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err = cmd.Run()
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("attempt=%d code=%d", attempt, exitErr.ExitCode())
		}

		if err == nil {
			return nil
		}

		log.Printf("Attempt %d failed: %v %T", attempt+1, err, err)
		log.Printf("Retrying in %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
	}

	return fmt.Errorf("script failed after 5 attempts: %w", err)
}
