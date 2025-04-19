package lib

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dnsbl"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/web"
	"github.com/TecharoHQ/anubis/xess"
)

var (
	challengesIssued = promauto.NewCounter(prometheus.CounterOpts{
		Name: "anubis_challenges_issued",
		Help: "The total number of challenges issued",
	})

	challengesValidated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "anubis_challenges_validated",
		Help: "The total number of challenges validated",
	})

	droneBLHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_dronebl_hits",
		Help: "The total number of hits from DroneBL",
	}, []string{"status"})

	failedValidations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "anubis_failed_validations",
		Help: "The total number of failed validations",
	})

	timeTaken = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "anubis_time_taken",
		Help:    "The time taken for a browser to generate a response (milliseconds)",
		Buckets: prometheus.ExponentialBucketsRange(1, math.Pow(2, 18), 19),
	})
)

type Options struct {
	Next            http.Handler
	Policy          *policy.ParsedConfig
	RedirectDomains []string
	ServeRobotsTXT  bool
	PrivateKey      ed25519.PrivateKey

	CookieDomain      string
	CookieName        string
	CookiePartitioned bool

	OGPassthrough bool
	OGTimeToLive  time.Duration
	Target        string

	WebmasterEmail string
	BasePrefix     string
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

	result := &Server{
		next:       opts.Next,
		priv:       opts.PrivateKey,
		pub:        opts.PrivateKey.Public().(ed25519.PublicKey),
		policy:     opts.Policy,
		opts:       opts,
		DNSBLCache: decaymap.New[string, dnsbl.DroneBLResponse](),
		OGTags:     ogtags.NewOGTagCache(opts.Target, opts.OGPassthrough, opts.OGTimeToLive),
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

	result.mux = mux

	return result, nil
}

type Server struct {
	mux        *http.ServeMux
	next       http.Handler
	priv       ed25519.PrivateKey
	pub        ed25519.PublicKey
	policy     *policy.ParsedConfig
	opts       Options
	DNSBLCache *decaymap.Impl[string, dnsbl.DroneBLResponse]
	OGTags     *ogtags.OGTagCache
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) ServeHTTPNext(w http.ResponseWriter, r *http.Request) {
	if s.next == nil {
		redir := r.FormValue("redir")
		urlParsed, err := r.URL.Parse(redir)
		if err != nil {
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect URL not parseable", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		}

		if len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host) {
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect domain not allowed", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		} else if urlParsed.Host != r.URL.Host {
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect domain not allowed", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		}

		if redir != "" {
			http.Redirect(w, r, redir, http.StatusFound)
			return
		}

		templ.Handler(
			web.Base("You are not a bot!", web.StaticHappy()),
		).ServeHTTP(w, r)
	} else {
		s.next.ServeHTTP(w, r)
	}
}

func (s *Server) challengeFor(r *http.Request, difficulty int) string {
	fp := sha256.Sum256(s.priv.Seed())

	challengeData := fmt.Sprintf(
		"Accept-Language=%s,X-Real-IP=%s,User-Agent=%s,WeekTime=%s,Fingerprint=%x,Difficulty=%d",
		r.Header.Get("Accept-Language"),
		r.Header.Get("X-Real-Ip"),
		r.UserAgent(),
		time.Now().UTC().Round(24*7*time.Hour).Format(time.RFC3339),
		fp,
		difficulty,
	)
	return internal.SHA256sum(challengeData)
}

func (s *Server) maybeReverseProxyHttpStatusOnly(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, true)
}

func (s *Server) maybeReverseProxyOrPage(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, false)
}

func (s *Server) maybeReverseProxy(w http.ResponseWriter, r *http.Request, httpStatusOnly bool) {
	lg := slog.With(
		"user_agent", r.UserAgent(),
		"accept_language", r.Header.Get("Accept-Language"),
		"priority", r.Header.Get("Priority"),
		"x-forwarded-for",
		r.Header.Get("X-Forwarded-For"),
		"x-real-ip", r.Header.Get("X-Real-Ip"),
	)

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"maybeReverseProxy\"", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	r.Header.Add("X-Anubis-Rule", cr.Name)
	r.Header.Add("X-Anubis-Action", string(cr.Rule))
	lg = lg.With("check_result", cr)
	policy.Applications.WithLabelValues(cr.Name, string(cr.Rule)).Add(1)

	ip := r.Header.Get("X-Real-Ip")

	if s.policy.DNSBL && ip != "" {
		resp, ok := s.DNSBLCache.Get(ip)
		if !ok {
			lg.Debug("looking up ip in dnsbl")
			resp, err := dnsbl.Lookup(ip)
			if err != nil {
				lg.Error("can't look up ip in dnsbl", "err", err)
			}
			s.DNSBLCache.Set(ip, resp, 24*time.Hour)
			droneBLHits.WithLabelValues(resp.String()).Inc()
		}

		if resp != dnsbl.AllGood {
			lg.Info("DNSBL hit", "status", resp.String())
			templ.Handler(web.Base("Oh noes!", web.ErrorPage(fmt.Sprintf("DroneBL reported an entry: %s, see https://dronebl.org/lookup?ip=%s", resp.String(), ip), s.opts.WebmasterEmail)), templ.WithStatus(http.StatusOK)).ServeHTTP(w, r)
			return
		}
	}

	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("allowing traffic to origin (explicit)")
		s.ServeHTTPNext(w, r)
		return
	case config.RuleDeny:
		s.ClearCookie(w)
		lg.Info("explicit deny")
		if rule == nil {
			lg.Error("rule is nil, cannot calculate checksum")
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		}
		hash := rule.Hash()

		lg.Debug("rule hash", "hash", hash)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage(fmt.Sprintf("Access Denied: error code %s", hash), s.opts.WebmasterEmail)), templ.WithStatus(http.StatusOK)).ServeHTTP(w, r)
		return
	case config.RuleChallenge:
		lg.Debug("challenge requested")
	case config.RuleBenchmark:
		lg.Debug("serving benchmark page")
		s.RenderBench(w, r)
		return
	default:
		s.ClearCookie(w)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	ckie, err := r.Cookie(anubis.CookieName)
	if err != nil {
		lg.Debug("cookie not found", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if err := ckie.Valid(); err != nil {
		lg.Debug("cookie is invalid", "err", err)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if time.Now().After(ckie.Expires) && !ckie.Expires.IsZero() {
		lg.Debug("cookie expired", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	token, err := jwt.ParseWithClaims(ckie.Value, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.pub, nil
	}, jwt.WithExpirationRequired(), jwt.WithStrictDecoding())

	if err != nil || !token.Valid {
		lg.Debug("invalid token", "path", r.URL.Path, "err", err)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if randomJitter() {
		r.Header.Add("X-Anubis-Status", "PASS-BRIEF")
		lg.Debug("cookie is not enrolled into secondary screening")
		s.ServeHTTPNext(w, r)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		lg.Debug("invalid token claims type", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}
	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	if claims["challenge"] != challenge {
		lg.Debug("invalid challenge", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	var nonce int

	if v, ok := claims["nonce"].(float64); ok {
		nonce = int(v)
	}

	calcString := fmt.Sprintf("%s%d", challenge, nonce)
	calculated := internal.SHA256sum(calcString)

	if subtle.ConstantTimeCompare([]byte(claims["response"].(string)), []byte(calculated)) != 1 {
		lg.Debug("invalid response", "path", r.URL.Path)
		failedValidations.Inc()
		s.ClearCookie(w)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	slog.Debug("all checks passed")
	r.Header.Add("X-Anubis-Status", "PASS-FULL")
	s.ServeHTTPNext(w, r)
}

func (s *Server) RenderIndex(w http.ResponseWriter, r *http.Request, rule *policy.Bot, returnHTTPStatusOnly bool) {
	if returnHTTPStatusOnly {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Authorization required"))
		return
	}

	lg := slog.With(
		"user_agent", r.UserAgent(),
		"accept_language", r.Header.Get("Accept-Language"),
		"priority", r.Header.Get("Priority"),
		"x-forwarded-for",
		r.Header.Get("X-Forwarded-For"),
		"x-real-ip", r.Header.Get("X-Real-Ip"),
	)

	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	var ogTags map[string]string = nil
	if s.opts.OGPassthrough {
		var err error
		ogTags, err = s.OGTags.GetOGTags(r.URL)
		if err != nil {
			lg.Error("failed to get OG tags", "err", err)
			ogTags = nil
		}
	}

	component, err := web.BaseWithChallengeAndOGTags("Making sure you're not a bot!", web.Index(), challenge, rule.Challenge, ogTags)
	if err != nil {
		lg.Error("render failed", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	handler := internal.NoStoreCache(templ.Handler(component))
	handler.ServeHTTP(w, r)
}

func (s *Server) RenderBench(w http.ResponseWriter, r *http.Request) {
	templ.Handler(
		web.Base("Benchmarking Anubis!", web.Bench()),
	).ServeHTTP(w, r)
}

func (s *Server) MakeChallenge(w http.ResponseWriter, r *http.Request) {
	lg := slog.With("user_agent", r.UserAgent(), "accept_language", r.Header.Get("Accept-Language"), "priority", r.Header.Get("Priority"), "x-forwarded-for", r.Header.Get("X-Forwarded-For"), "x-real-ip", r.Header.Get("X-Real-Ip"))

	encoder := json.NewEncoder(w)
	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		err := encoder.Encode(struct {
			Error string `json:"error"`
		}{
			Error: "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"makeChallenge\"",
		})
		if err != nil {
			lg.Error("failed to encode error response", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	lg = lg.With("check_result", cr)
	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	err = encoder.Encode(struct {
		Challenge string                 `json:"challenge"`
		Rules     *config.ChallengeRules `json:"rules"`
	}{
		Challenge: challenge,
		Rules:     rule.Challenge,
	})
	if err != nil {
		lg.Error("failed to encode challenge", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lg.Debug("made challenge", "challenge", challenge, "rules", rule.Challenge, "cr", cr)
	challengesIssued.Inc()
}

func (s *Server) PassChallenge(w http.ResponseWriter, r *http.Request) {
	lg := slog.With(
		"user_agent", r.UserAgent(),
		"accept_language", r.Header.Get("Accept-Language"),
		"priority", r.Header.Get("Priority"),
		"x-forwarded-for", r.Header.Get("X-Forwarded-For"),
		"x-real-ip", r.Header.Get("X-Real-Ip"),
	)

	redir := r.FormValue("redir")
	redirURL, err := url.ParseRequestURI(redir)
	if err != nil {
		lg.Error("invalid redirect", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid redirect", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}
	// used by the path checker rule
	r.URL = redirURL

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"passChallenge\".", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}
	lg = lg.With("check_result", cr)

	nonceStr := r.FormValue("nonce")
	if nonceStr == "" {
		s.ClearCookie(w)
		lg.Debug("no nonce")
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("missing nonce", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	elapsedTimeStr := r.FormValue("elapsedTime")
	if elapsedTimeStr == "" {
		s.ClearCookie(w)
		lg.Debug("no elapsedTime")
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("missing elapsedTime", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	elapsedTime, err := strconv.ParseFloat(elapsedTimeStr, 64)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("elapsedTime doesn't parse", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid elapsedTime", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	lg.Info("challenge took", "elapsedTime", elapsedTime)
	timeTaken.Observe(elapsedTime)

	response := r.FormValue("response")
	urlParsed, err := r.URL.Parse(redir)
	if err != nil {
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect URL not parseable", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	if len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host) {
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect domain not allowed", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	} else if urlParsed.Host != r.URL.Host {
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Redirect domain not allowed", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	nonce, err := strconv.Atoi(nonceStr)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("nonce doesn't parse", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid nonce", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	calcString := fmt.Sprintf("%s%d", challenge, nonce)
	calculated := internal.SHA256sum(calcString)

	if subtle.ConstantTimeCompare([]byte(response), []byte(calculated)) != 1 {
		s.ClearCookie(w)
		lg.Debug("hash does not match", "got", response, "want", calculated)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid response", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusForbidden)).ServeHTTP(w, r)
		failedValidations.Inc()
		return
	}

	// compare the leading zeroes
	if !strings.HasPrefix(response, strings.Repeat("0", rule.Challenge.Difficulty)) {
		s.ClearCookie(w)
		lg.Debug("difficulty check failed", "response", response, "difficulty", rule.Challenge.Difficulty)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid response", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusForbidden)).ServeHTTP(w, r)
		failedValidations.Inc()
		return
	}

	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}
	// generate JWT cookie
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"challenge": challenge,
		"nonce":     nonce,
		"response":  response,
		"iat":       time.Now().Unix(),
		"nbf":       time.Now().Add(-1 * time.Minute).Unix(),
		"exp":       time.Now().Add(24 * 7 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(s.priv)
	if err != nil {
		lg.Error("failed to sign JWT", "err", err)
		s.ClearCookie(w)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("failed to sign JWT", s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:        anubis.CookieName,
		Value:       tokenString,
		Expires:     time.Now().Add(24 * 7 * time.Hour),
		SameSite:    http.SameSiteLaxMode,
		Domain:      s.opts.CookieDomain,
		Partitioned: s.opts.CookiePartitioned,
		Path:        cookiePath,
	})

	challengesValidated.Inc()
	lg.Debug("challenge passed, redirecting to app")
	http.Redirect(w, r, redir, http.StatusFound)
}

func (s *Server) TestError(w http.ResponseWriter, r *http.Request) {
	err := r.FormValue("err")
	templ.Handler(web.Base("Oh noes!", web.ErrorPage(err, s.opts.WebmasterEmail)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
}

func cr(name string, rule config.Rule) policy.CheckResult {
	return policy.CheckResult{
		Name: name,
		Rule: rule,
	}
}

// Check evaluates the list of rules, and returns the result
func (s *Server) check(r *http.Request) (policy.CheckResult, *policy.Bot, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return decaymap.Zilch[policy.CheckResult](), nil, fmt.Errorf("[misconfiguration] X-Real-Ip header is not set")
	}

	addr := net.ParseIP(host)
	if addr == nil {
		return decaymap.Zilch[policy.CheckResult](), nil, fmt.Errorf("[misconfiguration] %q is not an IP address", host)
	}

	for _, b := range s.policy.Bots {
		match, err := b.Rules.Check(r)
		if err != nil {
			return decaymap.Zilch[policy.CheckResult](), nil, fmt.Errorf("can't run check %s: %w", b.Name, err)
		}

		if match {
			return cr("bot/"+b.Name, b.Action), &b, nil
		}
	}

	return cr("default/allow", config.RuleAllow), &policy.Bot{
		Challenge: &config.ChallengeRules{
			Difficulty: s.policy.DefaultDifficulty,
			ReportAs:   s.policy.DefaultDifficulty,
			Algorithm:  config.AlgorithmFast,
		},
	}, nil
}

func (s *Server) CleanupDecayMap() {
	s.DNSBLCache.Cleanup()
	s.OGTags.Cleanup()
}
