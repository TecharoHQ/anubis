package lib

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dnsbl"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/TecharoHQ/anubis/xess"
)

type Options struct {
	Next                 http.Handler
	Policy               *policy.ParsedConfig
	Target               string
	CookieDomain         string
	CookieName           string
	BasePrefix           string
	WebmasterEmail       string
	RedirectDomains      []string
	PrivateKey           ed25519.PrivateKey
	CookieExpiration     time.Duration
	OGTimeToLive         time.Duration
	OGCacheConsidersHost bool
	OGPassthrough        bool
	CookiePartitioned    bool
	ServeRobotsTXT       bool
}

func LoadPoliciesOrDefault(fname string, defaultDifficulty int) (*policy.ParsedConfig, error) {
	var fin io.ReadCloser
	var err error

	if fname != "" {
		fin, err = os.Open(fname)
		if err != nil {
			return nil, fmt.Errorf("can't parse policy file %s: %w", fname, err)
		}
	} else {
		fname = "(data)/botPolicies.yaml"
		fin, err = data.BotPolicies.Open("botPolicies.yaml")
		if err != nil {
			return nil, fmt.Errorf("[unexpected] can't parse builtin policy file %s: %w", fname, err)
		}
	}

	defer func(fin io.ReadCloser) {
		err := fin.Close()
		if err != nil {
			slog.Error("failed to close policy file", "file", fname, "err", err)
		}
	}(fin)

	anubisPolicy, err := policy.ParseConfig(fin, fname, defaultDifficulty)
	var validationErrs []error

	for _, b := range anubisPolicy.Bots {
		if _, ok := challenge.Get(b.Challenge.Algorithm); !ok {
			validationErrs = append(validationErrs, fmt.Errorf("%w %s", policy.ErrChallengeRuleHasWrongAlgorithm, b.Challenge.Algorithm))
		}
	}

	if len(validationErrs) != 0 {
		return nil, fmt.Errorf("can't do final validation of Anubis config: %w", errors.Join(validationErrs...))
	}

	return anubisPolicy, err
}

func New(opts Options) (*Server, error) {
	if opts.PrivateKey == nil {
		slog.Debug("opts.PrivateKey not set, generating a new one")
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("lib: can't generate private key: %v", err)
		}
		opts.PrivateKey = priv
	}

	anubis.BasePrefix = opts.BasePrefix

	cookieName := anubis.CookieName

	if opts.CookieDomain != "" {
		cookieName = anubis.WithDomainCookieName + opts.CookieDomain
	}

	result := &Server{
		next:       opts.Next,
		priv:       opts.PrivateKey,
		pub:        opts.PrivateKey.Public().(ed25519.PublicKey),
		policy:     opts.Policy,
		opts:       opts,
		DNSBLCache: decaymap.New[string, dnsbl.DroneBLResponse](),
		OGTags:     ogtags.NewOGTagCache(opts.Target, opts.OGPassthrough, opts.OGTimeToLive, opts.OGCacheConsidersHost),
		cookieName: cookieName,
	}

	mux := http.NewServeMux()
	xess.Mount(mux)

	// Helper to add global prefix
	registerWithPrefix := func(pattern string, handler http.Handler, method string) {
		if method != "" {
			method = method + " " // methods must end with a space to register with them
		}

		// Ensure there's no double slash when concatenating BasePrefix and pattern
		basePrefix := strings.TrimSuffix(anubis.BasePrefix, "/")
		prefix := method + basePrefix

		// If pattern doesn't start with a slash, add one
		if !strings.HasPrefix(pattern, "/") {
			pattern = "/" + pattern
		}

		mux.Handle(prefix+pattern, handler)
	}

	// Ensure there's no double slash when concatenating BasePrefix and StaticPath
	stripPrefix := strings.TrimSuffix(anubis.BasePrefix, "/") + anubis.StaticPath
	registerWithPrefix(anubis.StaticPath, internal.UnchangingCache(internal.NoBrowsing(http.StripPrefix(stripPrefix, http.FileServerFS(web.Static)))), "")

	if opts.ServeRobotsTXT {
		registerWithPrefix("/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		}), "GET")
		registerWithPrefix("/.well-known/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		}), "GET")
	}

	registerWithPrefix(anubis.APIPrefix+"make-challenge", http.HandlerFunc(result.MakeChallenge), "POST")
	registerWithPrefix(anubis.APIPrefix+"pass-challenge", http.HandlerFunc(result.PassChallenge), "GET")
	registerWithPrefix(anubis.APIPrefix+"check", http.HandlerFunc(result.maybeReverseProxyHttpStatusOnly), "")
	registerWithPrefix(anubis.APIPrefix+"test-error", http.HandlerFunc(result.TestError), "GET")
	registerWithPrefix("/", http.HandlerFunc(result.maybeReverseProxyOrPage), "")

	for _, implKind := range challenge.Methods() {
		impl, _ := challenge.Get(implKind)
		impl.Setup(mux)
	}

	result.mux = mux

	return result, nil
}
