package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/facebookgo/flagenv"
)

var (
	dir        = flag.String("dir", ".", "directory to serve")
	slogLevel  = flag.String("slog-level", "info", "logging level")
	socketPath = flag.String("socket-path", "./unixhttpd.sock", "unix socket path to use")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  %s [--dir=.] [--socket-path=./unixhttpd.sock]\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		os.Exit(2)
	}
}

func main() {
	flagenv.Parse()
	flag.Parse()

	internal.InitSlog(*slogLevel)

	if *dir == "" && *socketPath == "" {
		flag.Usage()
	}

	slog.Info("starting up", "dir", *dir, "socketPath", *socketPath)

	os.Remove(*socketPath)

	mux := http.NewServeMux()

	mux.HandleFunc("/reqmeta", func(w http.ResponseWriter, r *http.Request) {
		contains := strings.Contains(r.Header.Get("Accept"), "text/html")

		if contains {
			w.Header().Add("Content-Type", "text/html")
			fmt.Fprint(w, "<pre id=\"main\"><code>")
		}

		r.Write(w)

		if contains {
			fmt.Fprintln(w, "</pre></code>")
		}
	})

	mux.Handle("/", http.FileServer(http.Dir(*dir)))

	server := http.Server{
		Handler: mux,
	}

	unixListener, err := net.Listen("unix", *socketPath)
	if err != nil {
		panic(err)
	}
	log.Fatal(server.Serve(unixListener))
}
