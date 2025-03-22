package lib

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
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
	Next           http.Handler
	Policy         *policy.ParsedConfig
	ServeRobotsTXT bool
	PrivateKey     ed25519.PrivateKey

	CookieDomain      string
	CookieName        string
	CookiePartitioned bool
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
		fname = "(data)/botPolicies.json"
		fin, err = data.BotPolicies.Open("botPolicies.json")
		if err != nil {
			return nil, fmt.Errorf("[unexpected] can't parse builtin policy file %s: %w", fname, err)
		}
	}

	defer fin.Close()

	policy, err := policy.ParseConfig(fin, fname, defaultDifficulty)

	return policy, err
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

	result := &Server{
		next:       opts.Next,
		priv:       opts.PrivateKey,
		pub:        opts.PrivateKey.Public().(ed25519.PublicKey),
		policy:     opts.Policy,
		opts:       opts,
		DNSBLCache: decaymap.New[string, dnsbl.DroneBLResponse](),
	}

	mux := http.NewServeMux()
	xess.Mount(mux)

	mux.Handle(anubis.StaticPath, internal.UnchangingCache(internal.NoBrowsing(http.StripPrefix(anubis.StaticPath, http.FileServerFS(web.Static)))))

	if opts.ServeRobotsTXT {
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		})

		mux.HandleFunc("/.well-known/robots.txt", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, web.Static, "static/robots.txt")
		})
	}

	// mux.HandleFunc("GET /.within.website/x/cmd/anubis/static/js/main.mjs", serveMainJSWithBestEncoding)

	mux.HandleFunc("POST /.within.website/x/cmd/anubis/api/make-challenge", result.MakeChallenge)
	mux.HandleFunc("GET /.within.website/x/cmd/anubis/api/pass-challenge", result.PassChallenge)
	mux.HandleFunc("GET /.within.website/x/cmd/anubis/api/test-error", result.TestError)

	mux.HandleFunc("/", result.MaybeReverseProxy)

	result.mux = mux

	return result, nil
}

type Server struct {
	mux                 *http.ServeMux
	next                http.Handler
	priv                ed25519.PrivateKey
	pub                 ed25519.PublicKey
	policy              *policy.ParsedConfig
	opts                Options
	DNSBLCache          *decaymap.Impl[string, dnsbl.DroneBLResponse]
	ChallengeDifficulty int
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) challengeFor(r *http.Request, difficulty int) string {
	fp := sha256.Sum256(s.priv.Seed())

	data := fmt.Sprintf(
		"Accept-Language=%s,X-Real-IP=%s,User-Agent=%s,WeekTime=%s,Fingerprint=%x,Difficulty=%d",
		r.Header.Get("Accept-Language"),
		r.Header.Get("X-Real-Ip"),
		r.UserAgent(),
		time.Now().UTC().Round(24*7*time.Hour).Format(time.RFC3339),
		fp,
		difficulty,
	)
	return internal.SHA256sum(data)
}

func (s *Server) MaybeReverseProxy(w http.ResponseWriter, r *http.Request) {
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
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"maybeReverseProxy\"")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	r.Header.Add("X-Anubis-Rule", cr.Name)
	r.Header.Add("X-Anubis-Action", string(cr.Rule))
	lg = lg.With("check_result", cr)
	policy.PolicyApplications.WithLabelValues(cr.Name, string(cr.Rule)).Add(1)

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
			templ.Handler(web.Base("Oh noes!", web.ErrorPage(fmt.Sprintf("DroneBL reported an entry: %s, see https://dronebl.org/lookup?ip=%s", resp.String(), ip))), templ.WithStatus(http.StatusOK)).ServeHTTP(w, r)
			return
		}
	}

	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("allowing traffic to origin (explicit)")
		s.next.ServeHTTP(w, r)
		return
	case config.RuleDeny:
		s.ClearCookie(w)
		lg.Info("explicit deny")
		if rule == nil {
			lg.Error("rule is nil, cannot calculate checksum")
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		}
		hash, err := rule.Hash()
		if err != nil {
			lg.Error("can't calculate checksum of rule", "err", err)
			templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
			return
		}
		lg.Debug("rule hash", "hash", hash)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage(fmt.Sprintf("Access Denied: error code %s", hash))), templ.WithStatus(http.StatusOK)).ServeHTTP(w, r)
		return
	case config.RuleChallenge:
		lg.Debug("challenge requested")
	default:
		s.ClearCookie(w)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Other internal server error (contact the admin)")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	ckie, err := r.Cookie(anubis.CookieName)
	if err != nil {
		lg.Debug("cookie not found", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
		return
	}

	if err := ckie.Valid(); err != nil {
		lg.Debug("cookie is invalid", "err", err)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
		return
	}

	if time.Now().After(ckie.Expires) && !ckie.Expires.IsZero() {
		lg.Debug("cookie expired", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
		return
	}

	token, err := jwt.ParseWithClaims(ckie.Value, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.pub, nil
	}, jwt.WithExpirationRequired(), jwt.WithStrictDecoding())

	if err != nil || !token.Valid {
		lg.Debug("invalid token", "path", r.URL.Path, "err", err)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
		return
	}

	if randomJitter() {
		r.Header.Add("X-Anubis-Status", "PASS-BRIEF")
		lg.Debug("cookie is not enrolled into secondary screening")
		s.next.ServeHTTP(w, r)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		lg.Debug("invalid token claims type", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
		return
	}
	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	if claims["challenge"] != challenge {
		lg.Debug("invalid challenge", "path", r.URL.Path)
		s.ClearCookie(w)
		s.RenderIndex(w, r)
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
		s.RenderIndex(w, r)
		return
	}

	slog.Debug("all checks passed")
	r.Header.Add("X-Anubis-Status", "PASS-FULL")
	s.next.ServeHTTP(w, r)
}

func (s *Server) RenderIndex(w http.ResponseWriter, r *http.Request) {
	templ.Handler(
		web.Base("Making sure you're not a bot!", web.Index()),
	).ServeHTTP(w, r)
}

func (s *Server) MakeChallenge(w http.ResponseWriter, r *http.Request) {
	lg := slog.With("user_agent", r.UserAgent(), "accept_language", r.Header.Get("Accept-Language"), "priority", r.Header.Get("Priority"), "x-forwarded-for", r.Header.Get("X-Forwarded-For"), "x-real-ip", r.Header.Get("X-Real-Ip"))

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(struct {
			Error string `json:"error"`
		}{
			Error: "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"makeChallenge\"",
		})
		return
	}
	lg = lg.With("check_result", cr)
	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	json.NewEncoder(w).Encode(struct {
		Challenge string                 `json:"challenge"`
		Rules     *config.ChallengeRules `json:"rules"`
	}{
		Challenge: challenge,
		Rules:     rule.Challenge,
	})
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

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"passChallenge\".")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}
	lg = lg.With("check_result", cr)

	nonceStr := r.FormValue("nonce")
	if nonceStr == "" {
		s.ClearCookie(w)
		lg.Debug("no nonce")
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("missing nonce")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	elapsedTimeStr := r.FormValue("elapsedTime")
	if elapsedTimeStr == "" {
		s.ClearCookie(w)
		lg.Debug("no elapsedTime")
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("missing elapsedTime")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	elapsedTime, err := strconv.ParseFloat(elapsedTimeStr, 64)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("elapsedTime doesn't parse", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid elapsedTime")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	lg.Info("challenge took", "elapsedTime", elapsedTime)
	timeTaken.Observe(elapsedTime)

	response := r.FormValue("response")
	redir := r.FormValue("redir")

	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	nonce, err := strconv.Atoi(nonceStr)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("nonce doesn't parse", "err", err)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid nonce")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	calcString := fmt.Sprintf("%s%d", challenge, nonce)
	calculated := internal.SHA256sum(calcString)

	if subtle.ConstantTimeCompare([]byte(response), []byte(calculated)) != 1 {
		s.ClearCookie(w)
		lg.Debug("hash does not match", "got", response, "want", calculated)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid response")), templ.WithStatus(http.StatusForbidden)).ServeHTTP(w, r)
		failedValidations.Inc()
		return
	}

	// compare the leading zeroes
	if !strings.HasPrefix(response, strings.Repeat("0", s.ChallengeDifficulty)) {
		s.ClearCookie(w)
		lg.Debug("difficulty check failed", "response", response, "difficulty", s.ChallengeDifficulty)
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("invalid response")), templ.WithStatus(http.StatusForbidden)).ServeHTTP(w, r)
		failedValidations.Inc()
		return
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
		templ.Handler(web.Base("Oh noes!", web.ErrorPage("failed to sign JWT")), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:        anubis.CookieName,
		Value:       tokenString,
		Expires:     time.Now().Add(24 * 7 * time.Hour),
		SameSite:    http.SameSiteLaxMode,
		Domain:      s.opts.CookieDomain,
		Partitioned: s.opts.CookiePartitioned,
		Path:        "/",
	})

	challengesValidated.Inc()
	lg.Debug("challenge passed, redirecting to app")
	http.Redirect(w, r, redir, http.StatusFound)
}

func (s *Server) TestError(w http.ResponseWriter, r *http.Request) {
	err := r.FormValue("err")
	templ.Handler(web.Base("Oh noes!", web.ErrorPage(err)), templ.WithStatus(http.StatusInternalServerError)).ServeHTTP(w, r)
}

// Check evaluates the list of rules, and returns the result
func (s *Server) check(r *http.Request) (CheckResult, *policy.Bot, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return decaymap.Zilch[CheckResult](), nil, fmt.Errorf("[misconfiguration] X-Real-Ip header is not set")
	}

	addr := net.ParseIP(host)
	if addr == nil {
		return decaymap.Zilch[CheckResult](), nil, fmt.Errorf("[misconfiguration] %q is not an IP address", host)
	}

	for _, b := range s.policy.Bots {
		if b.UserAgent != nil {
			if b.UserAgent.MatchString(r.UserAgent()) && s.checkRemoteAddress(b, addr) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
		}

		if b.Path != nil {
			if b.Path.MatchString(r.URL.Path) && s.checkRemoteAddress(b, addr) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
		}

		if b.Ranger != nil {
			if s.checkRemoteAddress(b, addr) {
				return cr("bot/"+b.Name, b.Action), &b, nil
			}
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

func (s *Server) checkRemoteAddress(b policy.Bot, addr net.IP) bool {
	if b.Ranger == nil {
		return true
	}

	ok, err := b.Ranger.Contains(addr)
	if err != nil {
		log.Panicf("[unexpected] something very funky is going on, %q does not have a calculable network number: %v", addr.String(), err)
	}

	return ok
}
