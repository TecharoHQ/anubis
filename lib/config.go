package lib

import (
	"context"
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
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/honeypot/naive"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/TecharoHQ/anubis/xess"
	"github.com/a-h/templ"
)

type Options struct {
	Next                     http.Handler
	Policy                   *policy.ParsedConfig
	Target                   string
	TargetHost               string
	TargetSNI                string
	TargetInsecureSkipVerify bool
	CookieDynamicDomain      bool
	CookieDomain             string
	CookieExpiration         time.Duration
	CookiePartitioned        bool
	BasePrefix               string
	WebmasterEmail           string
	RedirectDomains          []string
	ED25519PrivateKey        ed25519.PrivateKey
	HS512Secret              []byte
	StripBasePrefix          bool
	OpenGraph                config.OpenGraph
	ServeRobotsTXT           bool
	CookieSecure             bool
	CookieSameSite           http.SameSite
	Logger                   *slog.Logger
	LogLevel                 string
	PublicUrl                string
	JWTRestrictionHeader     string
	DifficultyInJWT          bool
}

func LoadPoliciesOrDefault(ctx context.Context, fname string, defaultDifficulty int, logLevel string, subrequestMode bool) (*policy.ParsedConfig, error) {
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

	anubisPolicy, err := policy.ParseConfig(ctx, fin, fname, defaultDifficulty, logLevel, subrequestMode)
	if err != nil {
		return nil, fmt.Errorf("can't parse policy file %s: %w", fname, err)
	}
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
	if opts.Logger == nil {
		opts.Logger = slog.With("subsystem", "anubis")
	}

	if opts.ED25519PrivateKey == nil && opts.HS512Secret == nil {
		opts.Logger.Debug("opts.PrivateKey not set, generating a new one")
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("lib: can't generate private key: %v", err)
		}
		opts.ED25519PrivateKey = priv
	}

	opts.BasePrefix = strings.TrimRight(opts.BasePrefix, "/")

	result := &Server{
		next:        opts.Next,
		ed25519Priv: opts.ED25519PrivateKey,
		hs512Secret: opts.HS512Secret,
		policy:      opts.Policy,
		opts:        opts,
		basePrefix:  opts.BasePrefix,
		publicURL:   opts.PublicUrl,
		OGTags: ogtags.NewOGTagCache(opts.Target, opts.Policy.OpenGraph, opts.Policy.Store, ogtags.TargetOptions{
			Host:               opts.TargetHost,
			SNI:                opts.TargetSNI,
			InsecureSkipVerify: opts.TargetInsecureSkipVerify,
		}),
		store:  opts.Policy.Store,
		logger: opts.Logger,
	}

	mux := http.NewServeMux()
	xessPrefix := result.prefixedPath(xess.BasePrefix)
	mux.Handle(xessPrefix, internal.UnchangingCache(http.StripPrefix(xessPrefix, http.FileServerFS(xess.Static))))

	// Helper to add the server-local base prefix.
	registerWithPrefix := func(pattern string, handler http.Handler, method string) {
		if method != "" {
			method = method + " " // methods must end with a space to register with them
		}

		mux.Handle(method+result.prefixedPath(pattern), handler)
	}

	// Ensure there's no double slash when concatenating BasePrefix and StaticPath
	stripPrefix := result.prefixedPath(anubis.StaticPath)
	registerWithPrefix(anubis.StaticPath, internal.UnchangingCache(internal.NoBrowsing(http.StripPrefix(stripPrefix, http.FileServerFS(web.Static)))), "")

	if opts.ServeRobotsTXT {
		registerWithPrefix("/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		}), "GET")
		registerWithPrefix("/.well-known/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		}), "GET")
	}

	if opts.Policy.Impressum != nil {
		registerWithPrefix(anubis.APIPrefix+"imprint", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler := templ.Handler(
				web.BaseWithOptions(result.renderOptions(), opts.Policy.Impressum.Page.Title, opts.Policy.Impressum.Page, opts.Policy.Impressum, localization.GetLocalizer(r)),
			)
			handler.ServeHTTP(w, r)
		}), "GET")
	}

	registerWithPrefix(anubis.APIPrefix+"pass-challenge", http.HandlerFunc(result.PassChallenge), "GET")
	registerWithPrefix(anubis.APIPrefix+"check", http.HandlerFunc(result.maybeReverseProxyHttpStatusOnly), "")
	registerWithPrefix("/", http.HandlerFunc(result.maybeReverseProxyOrPage), "")

	mazeGen, err := naive.New(result.store, result.logger)
	if err == nil {
		registerWithPrefix(anubis.APIPrefix+"honeypot/{id}/{stage}", mazeGen, http.MethodGet)

		opts.Policy.Bots = append(
			opts.Policy.Bots,
			policy.Bot{
				Rules:  mazeGen.CheckNetwork(),
				Action: config.RuleWeigh,
				Weight: &config.Weight{
					Adjust: 30,
				},
				Name: "honeypot/network",
			},
		)
	} else {
		result.logger.Error("can't init honeypot subsystem", "err", err)
	}

	//goland:noinspection GoBoolExpressions
	if anubis.Version == "devel" {
		// make-challenge is only used in tests. Only enable while version is devel
		registerWithPrefix(anubis.APIPrefix+"make-challenge", http.HandlerFunc(result.MakeChallenge), "POST")
	}

	for _, implKind := range challenge.Methods() {
		impl, _ := challenge.Get(implKind)
		impl.Setup(mux)
	}

	result.mux = mux

	return result, nil
}
