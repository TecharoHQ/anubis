package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/web"
	"github.com/TecharoHQ/anubis/xess"
	"github.com/a-h/templ"
	"github.com/facebookgo/flagenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	bind                = flag.String("bind", ":8923", "network address to bind HTTP to")
	bindNetwork         = flag.String("bind-network", "tcp", "network family to bind HTTP to, e.g. unix, tcp")
	challengeDifficulty = flag.Int("difficulty", anubis.DefaultDifficulty, "difficulty of the challenge")
	metricsBind         = flag.String("metrics-bind", ":9090", "network address to bind metrics to")
	metricsBindNetwork  = flag.String("metrics-bind-network", "tcp", "network family for the metrics server to bind to")
	socketMode          = flag.String("socket-mode", "0770", "socket mode (permissions) for unix domain sockets.")
	robotsTxt           = flag.Bool("serve-robots-txt", false, "serve a robots.txt file that disallows all robots")
	policyFname         = flag.String("policy-fname", "", "full path to anubis policy document (defaults to a sensible built-in policy)")
	slogLevel           = flag.String("slog-level", "INFO", "logging level (see https://pkg.go.dev/log/slog#hdr-Levels)")
	target              = flag.String("target", "http://localhost:3923", "target to reverse proxy to")
	healthcheck         = flag.Bool("healthcheck", false, "run a health check against Anubis")
	debugXRealIPDefault = flag.String("debug-x-real-ip-default", "", "If set, replace empty X-Real-Ip headers with this value, useful only for debugging Anubis and running it locally")
)

func doHealthCheck() error {
	resp, err := http.Get("http://localhost" + *metricsBind + "/metrics")
	if err != nil {
		return fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func setupListener(network string, address string) (net.Listener, string) {
	formattedAddress := ""
	switch network {
	case "unix":
		formattedAddress = "unix:" + address
	case "tcp":
		formattedAddress = "http://localhost" + address
	default:
		formattedAddress = fmt.Sprintf(`(%s) %s`, network, address)
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to bind to %s: %w", formattedAddress, err))
	}

	// additional permission handling for unix sockets
	if network == "unix" {
		mode, err := strconv.ParseUint(*socketMode, 8, 0)
		if err != nil {
			listener.Close()
			log.Fatal(fmt.Errorf("could not parse socket mode %s: %w", *socketMode, err))
		}

		err = os.Chmod(address, os.FileMode(mode))
		if err != nil {
			listener.Close()
			log.Fatal(fmt.Errorf("could not change socket mode: %w", err))
		}
	}

	return listener, formattedAddress
}

func main() {
	flagenv.Parse()
	flag.Parse()

	internal.InitSlog(*slogLevel)

	if *healthcheck {
		if err := doHealthCheck(); err != nil {
			log.Fatal(err)
		}
		return
	}

	s, err := lib.New(*target, *policyFname, *challengeDifficulty)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Rule error IDs:")
	for _, rule := range s.Policy.Bots {
		if rule.Action != config.RuleDeny {
			continue
		}

		hash, err := rule.Hash()
		if err != nil {
			log.Fatalf("can't calculate checksum of rule %s: %v", rule.Name, err)
		}

		fmt.Printf("* %s: %s\n", rule.Name, hash)
	}
	fmt.Println()

	mux := http.NewServeMux()
	xess.Mount(mux)

	mux.Handle(anubis.StaticPath, internal.UnchangingCache(http.StripPrefix(anubis.StaticPath, http.FileServerFS(web.Static))))

	// mux.HandleFunc("GET /.within.website/x/cmd/anubis/static/js/main.mjs", serveMainJSWithBestEncoding)

	mux.HandleFunc("POST /.within.website/x/cmd/anubis/api/make-challenge", s.MakeChallenge)
	mux.HandleFunc("GET /.within.website/x/cmd/anubis/api/pass-challenge", s.PassChallenge)
	mux.HandleFunc("GET /.within.website/x/cmd/anubis/api/test-error", s.TestError)

	if *robotsTxt {
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		})

		mux.HandleFunc("/.well-known/robots.txt", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		})
	}

	wg := new(sync.WaitGroup)
	// install signal handler
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *metricsBind != "" {
		wg.Add(1)
		go metricsServer(ctx, wg.Done)
	}

	mux.HandleFunc("/", s.MaybeReverseProxy)

	var h http.Handler
	h = mux
	h = internal.DefaultXRealIP(*debugXRealIPDefault, h)
	h = internal.XForwardedForToXRealIP(h)

	srv := http.Server{Handler: h}
	listener, url := setupListener(*bindNetwork, *bind)
	slog.Info(
		"listening",
		"url", url,
		"difficulty", *challengeDifficulty,
		"serveRobotsTXT", *robotsTxt,
		"target", *target,
		"version", anubis.Version,
		"debug-x-real-ip-default", *debugXRealIPDefault,
	)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			log.Printf("cannot shut down: %v", err)
		}
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	wg.Wait()
}

func metricsServer(ctx context.Context, done func()) {
	defer done()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := http.Server{Handler: mux}
	listener, url := setupListener(*metricsBindNetwork, *metricsBind)
	slog.Debug("listening for metrics", "url", url)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			log.Printf("cannot shut down: %v", err)
		}
	}()

	if err := srv.Serve(listener); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func ohNoes(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("super fatal error", "err", err)
	templ.Handler(web.Base("Oh noes!", web.ErrorPage("An internal server error happened")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
}

func serveMainJSWithBestEncoding(w http.ResponseWriter, r *http.Request) {
	priorityList := []string{"zstd", "br", "gzip"}
	enc2ext := map[string]string{
		"zstd": "zst",
		"br":   "br",
		"gzip": "gz",
	}

	for _, enc := range priorityList {
		if strings.Contains(r.Header.Get("Accept-Encoding"), enc) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Header().Set("Content-Encoding", enc)
			http.ServeFileFS(w, r, web.Static, "static/js/main.mjs."+enc2ext[enc])
			return
		}
	}

	w.Header().Set("Content-Type", "text/javascript")
	http.ServeFileFS(w, r, web.Static, "static/js/main.mjs")
}
