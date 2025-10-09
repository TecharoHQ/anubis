package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

var (
	bind   = flag.String("bind", ":3000", "TCP port to bind to")
	silent = flag.Bool("silent", false, "if set, don't log http headers")
)

func main() {
	flag.Parse()

	slog.Info("listening", "url", "http://localhost"+*bind)
	log.Fatal(http.ListenAndServe(*bind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.within/health" {
			fmt.Fprintln(w, "OK")
			return
		}

		contains := strings.Contains(r.Header.Get("Accept"), "text/html")
		slog.Info("got request", "method", r.Method, "path", r.RequestURI)

		if contains {
			w.Header().Add("Content-Type", "text/html")
			fmt.Fprint(w, "<pre id=\"main\"><code>")
		}

		var out io.Writer

		switch *silent {
		case true:
			out = w
		case false:
			out = io.MultiWriter(w, os.Stdout)
		}

		fmt.Fprintln(out, r.Method, r.RequestURI)
		r.Header.Write(out)

		if contains {
			fmt.Fprintln(w, "</pre></code>")
		}
	})))
}
