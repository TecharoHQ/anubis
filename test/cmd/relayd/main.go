package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/facebookgo/flagenv"
	"github.com/google/uuid"
)

var (
	bind      = flag.String("bind", ":3004", "port to listen on")
	certDir   = flag.String("cert-dir", "/xe/pki", "where to read mounted certificates from")
	certFname = flag.String("cert-fname", "cert.pem", "certificate filename")
	keyFname  = flag.String("key-fname", "key.pem", "key filename")
	proxyTo   = flag.String("proxy-to", "http://localhost:5000", "where to reverse proxy to")
	slogLevel = flag.String("slog-level", "info", "logging level")
)

func main() {
	flagenv.Parse()
	flag.Parse()

	internal.InitSlog(*slogLevel)

	slog.Info("starting",
		"bind", *bind,
		"cert-dir", *certDir,
		"cert-fname", *certFname,
		"key-fname", *keyFname,
		"proxy-to", *proxyTo,
	)

	cert := filepath.Join(*certDir, *certFname)
	key := filepath.Join(*certDir, *keyFname)

	st, err := os.Stat(cert)

	if err != nil {
		slog.Error("can't stat cert file", "certFname", cert)
		os.Exit(1)
	}

	lastModified := st.ModTime()

	go func(lm time.Time) {
		t := time.NewTicker(time.Hour)
		defer t.Stop()

		for range t.C {
			st, err := os.Stat(cert)
			if err != nil {
				slog.Error("can't stat file", "fname", cert, "err", err)
				continue
			}

			if st.ModTime().After(lm) {
				slog.Info("new cert detected", "oldTime", lm.Format(time.RFC3339), "newTime", st.ModTime().Format(time.RFC3339))
				os.Exit(0)
			}
		}
	}(lastModified)

	u, err := url.Parse(*proxyTo)
	if err != nil {
		log.Fatal(err)
	}

	h := httputil.NewSingleHostReverseProxy(u)

	if u.Scheme == "unix" {
		slog.Info("using unix socket proxy")

		h = &httputil.ReverseProxy{
			Director: func(r *http.Request) {
				r.URL.Scheme = "http"
				r.URL.Host = r.Host

				r.Header.Set("X-Forwarded-Proto", "https")
				r.Header.Set("X-Forwarded-Scheme", "https")
				r.Header.Set("X-Request-Id", uuid.NewString())
				r.Header.Set("X-Scheme", "https")

				remoteHost, remotePort, err := net.SplitHostPort(r.Host)
				if err == nil {
					r.Header.Set("X-Forwarded-Host", remoteHost)
					r.Header.Set("X-Forwarded-Port", remotePort)
				} else {
					r.Header.Set("X-Forwarded-Host", r.Host)
				}

				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err == nil {
					r.Header.Set("X-Real-Ip", host)
				}
			},
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", strings.TrimPrefix(*proxyTo, "unix://"))
				},
			},
		}
	}

	log.Fatal(
		http.ListenAndServeTLS(
			*bind,
			cert,
			key,
			h,
		),
	)
}
