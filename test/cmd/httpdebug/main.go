package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
)

var (
	bind = flag.String("bind", ":3923", "TCP port to bind to")
)

func main() {
	flag.Parse()

	slog.Info("listening", "url", "http://localhost"+*bind)
	log.Fatal(http.ListenAndServe(*bind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("got request", "method", r.Method, "path", r.RequestURI)

		fmt.Fprintln(w, r.Method, r.RequestURI)
		r.Header.Write(w)
	})))
}
