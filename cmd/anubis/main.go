package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/web"
	"github.com/facebookgo/flagenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	bind                 = flag.String("bind", ":8923", "network address to bind HTTP to")
	bindNetwork          = flag.String("bind-network", "tcp", "network family to bind HTTP to, e.g. unix, tcp")
	challengeDifficulty  = flag.Int("difficulty", anubis.DefaultDifficulty, "difficulty of the challenge")
	cookieDomain         = flag.String("cookie-domain", "", "if set, the top-level domain that the Anubis cookie will be valid for")
	cookiePartitioned    = flag.Bool("cookie-partitioned", false, "if true, sets the partitioned flag on Anubis cookies, enabling CHIPS support")
	ed25519PrivateKeyHex = flag.String("ed25519-private-key-hex", "", "private key used to sign JWTs, if not set a random one will be assigned")
	metricsBind          = flag.String("metrics-bind", ":9090", "network address to bind metrics to")
	metricsBindNetwork   = flag.String("metrics-bind-network", "tcp", "network family for the metrics server to bind to")
	socketMode           = flag.String("socket-mode", "0770", "socket mode (permissions) for unix domain sockets.")
	robotsTxt            = flag.Bool("serve-robots-txt", false, "serve a robots.txt file that disallows all robots")
	policyFname          = flag.String("policy-fname", "", "full path to anubis policy document (defaults to a sensible built-in policy)")
	slogLevel            = flag.String("slog-level", "INFO", "logging level (see https://pkg.go.dev/log/slog#hdr-Levels)")
	target               = flag.String("target", "http://localhost:3923", "target to reverse proxy to")
	healthcheck          = flag.Bool("healthcheck", false, "run a health check against Anubis")
	debugXRealIPDefault  = flag.String("debug-x-real-ip-default", "", "If set, replace empty X-Real-Ip headers with this value, useful only for debugging Anubis and running it locally")
)

func keyFromHex(value string) (ed25519.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("supplied key is not hex-encoded: %w", err)
	}

	if len(keyBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("supplied key is not %d bytes long, got %d bytes", ed25519.SeedSize, len(keyBytes))
	}

	return ed25519.NewKeyFromSeed(keyBytes), nil
}

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

func makeReverseProxy(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	// https://github.com/oauth2-proxy/oauth2-proxy/blob/4e2100a2879ef06aea1411790327019c1a09217c/pkg/upstream/http.go#L124
	if u.Scheme == "unix" {
		// clean path up so we don't use the socket path in proxied requests
		addr := u.Path
		u.Path = ""
		// tell transport how to dial unix sockets
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := net.Dialer{}
			return dialer.DialContext(ctx, "unix", addr)
		}
		// tell transport how to handle the unix url scheme
		transport.RegisterProtocol("unix", libanubis.UnixRoundTripper{Transport: transport})
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = transport

	return rp, nil
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

	rp, err := makeReverseProxy(*target)
	if err != nil {
		log.Fatalf("can't make reverse proxy: %v", err)
	}

	policy, err := libanubis.LoadPoliciesOrDefault(*policyFname, *challengeDifficulty)
	if err != nil {
		log.Fatalf("can't parse policy file: %v", err)
	}

	fmt.Println("Rule error IDs:")
	for _, rule := range policy.Bots {
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

	var priv ed25519.PrivateKey
	if *ed25519PrivateKeyHex == "" {
		_, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalf("failed to generate ed25519 key: %v", err)
		}

		slog.Warn("generating random key, Anubis will have strange behavior when multiple instances are behind the same load balancer target, for more information: see https://anubis.techaro.lol/docs/admin/installation#key-generation")
	} else {
		priv, err = keyFromHex(*ed25519PrivateKeyHex)
		if err != nil {
			log.Fatalf("failed to parse and validate ED25519_PRIVATE_KEY_HEX: %v", err)
		}
	}

	s, err := libanubis.New(libanubis.Options{
		Next:              rp,
		Policy:            policy,
		ServeRobotsTXT:    *robotsTxt,
		PrivateKey:        priv,
		CookieDomain:      *cookieDomain,
		CookiePartitioned: *cookiePartitioned,
	})
	if err != nil {
		log.Fatalf("can't construct libanubis.Server: %v", err)
	}

	wg := new(sync.WaitGroup)
	// install signal handler
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *metricsBind != "" {
		wg.Add(1)
		go metricsServer(ctx, wg.Done)
	}

	var h http.Handler
	h = s
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
