package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/internal"
	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/TecharoHQ/anubis/lib/config"
	botPolicy "github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/thoth"
	"github.com/TecharoHQ/anubis/web"
	"github.com/facebookgo/flagenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

var (
	basePrefix               = flag.String("base-prefix", "", "base prefix (root URL) the application is served under e.g. /myapp")
	bind                     = flag.String("bind", ":8923", "network address to bind HTTP to")
	bindNetwork              = flag.String("bind-network", "tcp", "network family to bind HTTP to, e.g. unix, tcp")
	challengeDifficulty      = flag.Int("difficulty", anubis.DefaultDifficulty, "difficulty of the challenge")
	cookieDomain             = flag.String("cookie-domain", "", "if set, the top-level domain that the Anubis cookie will be valid for")
	cookieDynamicDomain      = flag.Bool("cookie-dynamic-domain", false, "if set, automatically set the cookie Domain value based on the request domain")
	cookieExpiration         = flag.Duration("cookie-expiration-time", anubis.CookieDefaultExpirationTime, "The amount of time the authorization cookie is valid for")
	cookiePrefix             = flag.String("cookie-prefix", anubis.CookieName, "prefix for browser cookies created by Anubis")
	cookiePartitioned        = flag.Bool("cookie-partitioned", false, "if true, sets the partitioned flag on Anubis cookies, enabling CHIPS support")
	difficultyInJWT          = flag.Bool("difficulty-in-jwt", false, "if true, adds a difficulty field in the JWT claims")
	useSimplifiedExplanation = flag.Bool("use-simplified-explanation", false, "if true, replaces the text when clicking \"Why am I seeing this?\" with a more simplified text for a non-tech-savvy audience.")
	forcedLanguage           = flag.String("forced-language", "", "if set, this language is being used instead of the one from the request's Accept-Language header")
	hs512Secret              = flag.String("hs512-secret", "", "secret used to sign JWTs, uses ed25519 if not set")
	cookieSecure             = flag.Bool("cookie-secure", true, "if true, sets the secure flag on Anubis cookies")
	cookieSameSite           = flag.String("cookie-same-site", "None", "sets the same site option on Anubis cookies, will auto-downgrade None to Lax if cookie-secure is false. Valid values are None, Lax, Strict, and Default.")
	ed25519PrivateKeyHex     = flag.String("ed25519-private-key-hex", "", "private key used to sign JWTs, if not set a random one will be assigned")
	ed25519PrivateKeyHexFile = flag.String("ed25519-private-key-hex-file", "", "file name containing value for ed25519-private-key-hex")
	metricsBind              = flag.String("metrics-bind", ":9090", "network address to bind metrics to")
	metricsBindNetwork       = flag.String("metrics-bind-network", "tcp", "network family for the metrics server to bind to")
	socketMode               = flag.String("socket-mode", "0770", "socket mode (permissions) for unix domain sockets.")
	robotsTxt                = flag.Bool("serve-robots-txt", false, "serve a robots.txt file that disallows all robots")
	policyFname              = flag.String("policy-fname", "", "full path to anubis policy document (defaults to a sensible built-in policy)")
	redirectDomains          = flag.String("redirect-domains", "", "list of domains separated by commas which anubis is allowed to redirect to. Leaving this unset allows any domain.")
	slogLevel                = flag.String("slog-level", "INFO", "logging level (see https://pkg.go.dev/log/slog#hdr-Levels)")
	stripBasePrefix          = flag.Bool("strip-base-prefix", false, "if true, strips the base prefix from requests forwarded to the target server")
	target                   = flag.String("target", "http://localhost:3923", "target to reverse proxy to, set to an empty string to disable proxying when only using auth request")
	targetSNI                = flag.String("target-sni", "", "if set, TLS handshake hostname when forwarding requests to the target, if set to auto, use Host header")
	targetHost               = flag.String("target-host", "", "if set, the value of the Host header when forwarding requests to the target")
	targetInsecureSkipVerify = flag.Bool("target-insecure-skip-verify", false, "if true, skips TLS validation for the backend")
	targetDisableKeepAlive   = flag.Bool("target-disable-keepalive", false, "if true, disables HTTP keep-alive for the backend")
	healthcheck              = flag.Bool("healthcheck", false, "run a health check against Anubis")
	useRemoteAddress         = flag.Bool("use-remote-address", false, "read the client's IP address from the network request, useful for debugging and running Anubis on bare metal")
	debugBenchmarkJS         = flag.Bool("debug-benchmark-js", false, "respond to every request with a challenge for benchmarking hashrate")
	ogPassthrough            = flag.Bool("og-passthrough", false, "enable Open Graph tag passthrough")
	ogTimeToLive             = flag.Duration("og-expiry-time", 24*time.Hour, "Open Graph tag cache expiration time")
	ogCacheConsiderHost      = flag.Bool("og-cache-consider-host", false, "enable or disable the use of the host in the Open Graph tag cache")
	extractResources         = flag.String("extract-resources", "", "if set, extract the static resources to the specified folder")
	webmasterEmail           = flag.String("webmaster-email", "", "if set, displays webmaster's email on the reject page for appeals")
	versionFlag              = flag.Bool("version", false, "print Anubis version")
	publicUrl                = flag.String("public-url", "", "the externally accessible URL for this Anubis instance, used for constructing redirect URLs (e.g., for forwardAuth).")
	xffStripPrivate          = flag.Bool("xff-strip-private", true, "if set, strip private addresses from X-Forwarded-For")
	customRealIPHeader       = flag.String("custom-real-ip-header", "", "if set, read remote IP from header of this name (in case your environment doesn't set X-Real-IP header)")

	thothInsecure        = flag.Bool("thoth-insecure", false, "if set, connect to Thoth over plain HTTP/2, don't enable this unless support told you to")
	thothURL             = flag.String("thoth-url", "", "if set, URL for Thoth, the IP reputation database for Anubis")
	thothToken           = flag.String("thoth-token", "", "if set, API token for Thoth, the IP reputation database for Anubis")
	jwtRestrictionHeader = flag.String("jwt-restriction-header", "X-Real-IP", "If set, the JWT is only valid if the current value of this header matched the value when the JWT was created")
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
	resp, err := http.Get("http://localhost" + *metricsBind + "/healthz")
	if err != nil {
		return fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// parseBindNetFromAddr determine bind network and address based on the given network and address.
func parseBindNetFromAddr(address string) (string, string) {
	defaultScheme := "http://"
	if !strings.Contains(address, "://") {
		if strings.HasPrefix(address, ":") {
			address = defaultScheme + "localhost" + address
		} else {
			address = defaultScheme + address
		}
	}

	bindUri, err := url.Parse(address)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to parse bind URL: %w", err))
	}

	switch bindUri.Scheme {
	case "unix":
		return "unix", bindUri.Path
	case "tcp", "http", "https":
		return "tcp", bindUri.Host
	default:
		log.Fatal(fmt.Errorf("unsupported network scheme %s in address %s", bindUri.Scheme, address))
	}
	return "", address
}

func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(s) {
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		log.Fatalf("invalid cookie same-site mode: %s, valid values are None, Lax, Strict, and Default", s)
	}
	return http.SameSiteDefaultMode
}

func setupListener(network string, address string) (net.Listener, string) {
	formattedAddress := ""

	if network == "" {
		// keep compatibility
		network, address = parseBindNetFromAddr(address)
	}

	switch network {
	case "unix":
		formattedAddress = "unix:" + address
	case "tcp":
		if strings.HasPrefix(address, ":") { // assume it's just a port e.g. :4259
			formattedAddress = "http://localhost" + address
		} else {
			formattedAddress = "http://" + address
		}
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
			err := listener.Close()
			if err != nil {
				log.Printf("failed to close listener: %v", err)
			}
			log.Fatal(fmt.Errorf("could not change socket mode: %w", err))
		}
	}

	return listener, formattedAddress
}

func makeReverseProxy(target string, targetSNI string, targetHost string, insecureSkipVerify bool, targetDisableKeepAlive bool) (http.Handler, error) {
	targetUri, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	if targetDisableKeepAlive {
		transport.DisableKeepAlives = true
	}

	// https://github.com/oauth2-proxy/oauth2-proxy/blob/4e2100a2879ef06aea1411790327019c1a09217c/pkg/upstream/http.go#L124
	if targetUri.Scheme == "unix" {
		// clean path up so we don't use the socket path in proxied requests
		addr := targetUri.Path
		targetUri.Path = ""
		// tell transport how to dial unix sockets
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := net.Dialer{}
			return dialer.DialContext(ctx, "unix", addr)
		}
		// tell transport how to handle the unix url scheme
		transport.RegisterProtocol("unix", libanubis.UnixRoundTripper{Transport: transport})
	}

	if insecureSkipVerify || targetSNI != "" {
		transport.TLSClientConfig = &tls.Config{}
	}
	if insecureSkipVerify {
		slog.Warn("TARGET_INSECURE_SKIP_VERIFY is set to true, TLS certificate validation will not be performed", "target", target)
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	if targetSNI != "" && targetSNI != "auto" {
		transport.TLSClientConfig.ServerName = targetSNI
	}

	rp := httputil.NewSingleHostReverseProxy(targetUri)
	rp.Transport = transport

	if targetHost != "" || targetSNI == "auto" {
		originalDirector := rp.Director
		rp.Director = func(req *http.Request) {
			originalDirector(req)
			if targetHost != "" {
				req.Host = targetHost
			}
			if targetSNI == "auto" {
				transport.TLSClientConfig.ServerName = req.Host
			}
		}
	}

	return rp, nil
}

func main() {
	flagenv.Parse()
	flag.Parse()

	if *versionFlag {
		fmt.Println("Anubis", anubis.Version)
		return
	}

	internal.SetHealth("anubis", healthv1.HealthCheckResponse_NOT_SERVING)

	lg := internal.InitSlog(*slogLevel, os.Stderr)
	lg.Info("starting up Anubis")

	if *healthcheck {
		log.Println("running healthcheck")
		if err := doHealthCheck(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *extractResources != "" {
		if err := extractEmbedFS(data.BotPolicies, ".", *extractResources); err != nil {
			log.Fatal(err)
		}
		if err := extractEmbedFS(web.Static, "static", *extractResources); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Extracted embedded static files to %s\n", *extractResources)
		return
	}

	// install signal handler
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wg := new(sync.WaitGroup)

	if *metricsBind != "" {
		wg.Add(1)
		go metricsServer(ctx, *lg.With("subsystem", "metrics"), wg.Done)
	}

	var rp http.Handler
	// when using anubis via Systemd and environment variables, then it is not possible to set targe to an empty string but only to space
	if strings.TrimSpace(*target) != "" {
		var err error
		rp, err = makeReverseProxy(*target, *targetSNI, *targetHost, *targetInsecureSkipVerify, *targetDisableKeepAlive)
		if err != nil {
			log.Fatalf("can't make reverse proxy: %v", err)
		}
	}

	if *cookieDomain != "" && *cookieDynamicDomain {
		log.Fatalf("you can't set COOKIE_DOMAIN and COOKIE_DYNAMIC_DOMAIN at the same time")
	}

	// Thoth configuration
	switch {
	case *thothURL != "" && *thothToken == "":
		lg.Warn("THOTH_URL is set but no THOTH_TOKEN is set")
	case *thothURL == "" && *thothToken != "":
		lg.Warn("THOTH_TOKEN is set but no THOTH_URL is set")
	case *thothURL != "" && *thothToken != "":
		lg.Debug("connecting to Thoth")
		thothClient, err := thoth.New(ctx, *thothURL, *thothToken, *thothInsecure)
		if err != nil {
			log.Fatalf("can't dial thoth at %s: %v", *thothURL, err)
		}

		ctx = thoth.With(ctx, thothClient)
	}

	lg.Info("loading policy file", "fname", *policyFname)
	policy, err := libanubis.LoadPoliciesOrDefault(ctx, *policyFname, *challengeDifficulty, *slogLevel)
	if err != nil {
		log.Fatalf("can't parse policy file: %v", err)
	}
	lg = policy.Logger
	lg.Debug("swapped to new logger")
	slog.SetDefault(lg)

	// Warn if persistent storage is used without a configured signing key
	if policy.Store.IsPersistent() {
		if *hs512Secret == "" && *ed25519PrivateKeyHex == "" && *ed25519PrivateKeyHexFile == "" {
			lg.Warn("[misconfiguration] persistent storage backend is configured, but no private key is set. " +
				"Challenges will be invalidated when Anubis restarts. " +
				"Set HS512_SECRET, ED25519_PRIVATE_KEY_HEX, or ED25519_PRIVATE_KEY_HEX_FILE to ensure challenges survive service restarts. " +
				"See: https://anubis.techaro.lol/docs/admin/installation#key-generation")
		}
	}

	ruleErrorIDs := make(map[string]string)
	for _, rule := range policy.Bots {
		if rule.Action != config.RuleDeny {
			continue
		}

		hash := rule.Hash()
		ruleErrorIDs[rule.Name] = hash
	}

	// replace the bot policy rules with a single rule that always benchmarks
	if *debugBenchmarkJS {
		policy.Bots = []botPolicy.Bot{{
			Name:   "",
			Rules:  botPolicy.NewHeaderExistsChecker("User-Agent"),
			Action: config.RuleBenchmark,
		}}
	}
	if *basePrefix != "" && !strings.HasPrefix(*basePrefix, "/") {
		log.Fatalf("[misconfiguration] base-prefix must start with a slash, eg: /%s", *basePrefix)
	} else if strings.HasSuffix(*basePrefix, "/") {
		log.Fatalf("[misconfiguration] base-prefix must not end with a slash")
	}
	if *stripBasePrefix && *basePrefix == "" {
		log.Fatalf("[misconfiguration] strip-base-prefix is set to true, but base-prefix is not set, " +
			"this may result in unexpected behavior")
	}

	var ed25519Priv ed25519.PrivateKey
	if *hs512Secret != "" && (*ed25519PrivateKeyHex != "" || *ed25519PrivateKeyHexFile != "") {
		log.Fatal("do not specify both HS512 and ED25519 secrets")
	} else if *hs512Secret != "" {
		ed25519Priv = ed25519.PrivateKey(*hs512Secret)
	} else if *ed25519PrivateKeyHex != "" && *ed25519PrivateKeyHexFile != "" {
		log.Fatal("do not specify both ED25519_PRIVATE_KEY_HEX and ED25519_PRIVATE_KEY_HEX_FILE")
	} else if *ed25519PrivateKeyHex != "" {
		ed25519Priv, err = keyFromHex(*ed25519PrivateKeyHex)
		if err != nil {
			log.Fatalf("failed to parse and validate ED25519_PRIVATE_KEY_HEX: %v", err)
		}
	} else if *ed25519PrivateKeyHexFile != "" {
		hexFile, err := os.ReadFile(*ed25519PrivateKeyHexFile)
		if err != nil {
			log.Fatalf("failed to read ED25519_PRIVATE_KEY_HEX_FILE %s: %v", *ed25519PrivateKeyHexFile, err)
		}

		ed25519Priv, err = keyFromHex(string(bytes.TrimSpace(hexFile)))
		if err != nil {
			log.Fatalf("failed to parse and validate content of ED25519_PRIVATE_KEY_HEX_FILE: %v", err)
		}
	} else {
		_, ed25519Priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Fatalf("failed to generate ed25519 key: %v", err)
		}

		lg.Warn("generating random key, Anubis will have strange behavior when multiple instances are behind the same load balancer target, for more information: see https://anubis.techaro.lol/docs/admin/installation#key-generation")
	}

	var redirectDomainsList []string
	if *redirectDomains != "" {
		domains := strings.Split(*redirectDomains, ",")
		for _, domain := range domains {
			_, err = url.Parse(domain)
			if err != nil {
				log.Fatalf("cannot parse redirect-domain %q: %s", domain, err.Error())
			}
			redirectDomainsList = append(redirectDomainsList, strings.TrimSpace(domain))
		}
	} else {
		lg.Warn("REDIRECT_DOMAINS is not set, Anubis will only redirect to the same domain a request is coming from, see https://anubis.techaro.lol/docs/admin/configuration/redirect-domains")
	}

	anubis.CookieName = *cookiePrefix + "-auth"
	anubis.TestCookieName = *cookiePrefix + "-cookie-verification"
	anubis.ForcedLanguage = *forcedLanguage
	anubis.UseSimplifiedExplanation = *useSimplifiedExplanation

	// If OpenGraph configuration values are not set in the config file, use the
	// values from flags / envvars.
	if !policy.OpenGraph.Enabled {
		policy.OpenGraph.Enabled = *ogPassthrough
		policy.OpenGraph.ConsiderHost = *ogCacheConsiderHost
		policy.OpenGraph.TimeToLive = *ogTimeToLive
		policy.OpenGraph.Override = map[string]string{}
	}

	s, err := libanubis.New(libanubis.Options{
		BasePrefix:               *basePrefix,
		StripBasePrefix:          *stripBasePrefix,
		Next:                     rp,
		Policy:                   policy,
		TargetHost:               *targetHost,
		TargetSNI:                *targetSNI,
		TargetInsecureSkipVerify: *targetInsecureSkipVerify,
		ServeRobotsTXT:           *robotsTxt,
		ED25519PrivateKey:        ed25519Priv,
		HS512Secret:              []byte(*hs512Secret),
		CookieDomain:             *cookieDomain,
		CookieDynamicDomain:      *cookieDynamicDomain,
		CookieExpiration:         *cookieExpiration,
		CookiePartitioned:        *cookiePartitioned,
		RedirectDomains:          redirectDomainsList,
		Target:                   *target,
		WebmasterEmail:           *webmasterEmail,
		OpenGraph:                policy.OpenGraph,
		CookieSecure:             *cookieSecure,
		CookieSameSite:           parseSameSite(*cookieSameSite),
		PublicUrl:                *publicUrl,
		JWTRestrictionHeader:     *jwtRestrictionHeader,
		Logger:                   policy.Logger.With("subsystem", "anubis"),
		DifficultyInJWT:          *difficultyInJWT,
	})
	if err != nil {
		log.Fatalf("can't construct libanubis.Server: %v", err)
	}

	var h http.Handler
	h = s
	h = internal.CustomRealIPHeader(*customRealIPHeader, h)
	h = internal.RemoteXRealIP(*useRemoteAddress, *bindNetwork, h)
	h = internal.XForwardedForToXRealIP(h)
	h = internal.XForwardedForUpdate(*xffStripPrivate, h)
	h = internal.JA4H(h)

	srv := http.Server{Handler: h, ErrorLog: internal.GetFilteredHTTPLogger()}
	listener, listenerUrl := setupListener(*bindNetwork, *bind)
	lg.Info(
		"listening",
		"url", listenerUrl,
		"difficulty", *challengeDifficulty,
		"serveRobotsTXT", *robotsTxt,
		"target", *target,
		"version", anubis.Version,
		"use-remote-address", *useRemoteAddress,
		"debug-benchmark-js", *debugBenchmarkJS,
		"og-passthrough", *ogPassthrough,
		"og-expiry-time", *ogTimeToLive,
		"base-prefix", *basePrefix,
		"cookie-expiration-time", *cookieExpiration,
		"rule-error-ids", ruleErrorIDs,
		"public-url", *publicUrl,
	)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			log.Printf("cannot shut down: %v", err)
		}
	}()

	internal.SetHealth("anubis", healthv1.HealthCheckResponse_SERVING)

	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	wg.Wait()
}

func metricsServer(ctx context.Context, lg slog.Logger, done func()) {
	defer done()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		st, ok := internal.GetHealth("anubis")
		if !ok {
			slog.Error("health service anubis does not exist, file a bug")
		}

		switch st {
		case healthv1.HealthCheckResponse_NOT_SERVING:
			http.Error(w, "NOT OK", http.StatusInternalServerError)
			return
		case healthv1.HealthCheckResponse_SERVING:
			fmt.Fprintln(w, "OK")
			return
		default:
			http.Error(w, "UNKNOWN", http.StatusFailedDependency)
			return
		}
	})

	srv := http.Server{Handler: mux, ErrorLog: internal.GetFilteredHTTPLogger()}
	listener, metricsUrl := setupListener(*metricsBindNetwork, *metricsBind)
	lg.Debug("listening for metrics", "url", metricsUrl)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			log.Printf("cannot shut down: %v", err)
		}
	}()

	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func extractEmbedFS(fsys embed.FS, root string, destDir string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, root, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0o700)
		}

		embeddedData, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, embeddedData, 0o644)
	})
}
