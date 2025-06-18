package lib

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/cel-go/common/types"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dnsbl"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/TecharoHQ/anubis/lib/policy/config"

	// challenge implementations
	_ "github.com/TecharoHQ/anubis/lib/challenge/metarefresh"
	_ "github.com/TecharoHQ/anubis/lib/challenge/proofofwork"
)

var (
	challengesIssued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_challenges_issued",
		Help: "The total number of challenges issued",
	}, []string{"method"})

	challengesValidated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_challenges_validated",
		Help: "The total number of challenges validated",
	}, []string{"method"})

	droneBLHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_dronebl_hits",
		Help: "The total number of hits from DroneBL",
	}, []string{"status"})

	failedValidations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_failed_validations",
		Help: "The total number of failed validations",
	}, []string{"method"})

	requestsProxied = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "anubis_proxied_requests_total",
		Help: "Number of requests proxied through Anubis to upstream targets",
	}, []string{"host"})
)

type Server struct {
	next       http.Handler
	mux        *http.ServeMux
	policy     *policy.ParsedConfig
	DNSBLCache *decaymap.Impl[string, dnsbl.DroneBLResponse]
	OGTags     *ogtags.OGTagCache
	cookieName string
	priv       ed25519.PrivateKey
	pub        ed25519.PublicKey
	opts       Options
}

func (s *Server) challengeFor(r *http.Request, difficulty int) string {
	fp := sha256.Sum256(s.pub[:])

	acceptLanguage := r.Header.Get("Accept-Language")
	if len(acceptLanguage) > 5 {
		acceptLanguage = acceptLanguage[:5]
	}

	challengeData := fmt.Sprintf(
		"Accept-Language=%s,X-Real-IP=%s,User-Agent=%s,WeekTime=%s,Fingerprint=%x,Difficulty=%d",
		acceptLanguage,
		r.Header.Get("X-Real-Ip"),
		r.UserAgent(),
		time.Now().UTC().Round(24*7*time.Hour).Format(time.RFC3339),
		fp,
		difficulty,
	)
	return internal.FastHash(challengeData)
}

func (s *Server) maybeReverseProxyHttpStatusOnly(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, true)
}

func (s *Server) maybeReverseProxyOrPage(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, false)
}

func (s *Server) maybeReverseProxy(w http.ResponseWriter, r *http.Request, httpStatusOnly bool) {
	lg := internal.GetRequestLogger(r)

	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"maybeReverseProxy\"")
		return
	}

	r.Header.Add("X-Anubis-Rule", cr.Name)
	r.Header.Add("X-Anubis-Action", string(cr.Rule))
	lg = lg.With("check_result", cr)
	policy.Applications.WithLabelValues(cr.Name, string(cr.Rule)).Add(1)

	ip := r.Header.Get("X-Real-Ip")

	if s.handleDNSBL(w, r, ip, lg) {
		return
	}

	if s.checkRules(w, r, cr, lg, rule) {
		return
	}

	ckie, err := r.Cookie(s.cookieName)
	if err != nil {
		lg.Debug("cookie not found", "path", r.URL.Path)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if err := ckie.Valid(); err != nil {
		lg.Debug("cookie is invalid", "err", err)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if time.Now().After(ckie.Expires) && !ckie.Expires.IsZero() {
		lg.Debug("cookie expired", "path", r.URL.Path)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	token, err := jwt.ParseWithClaims(ckie.Value, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.pub, nil
	}, jwt.WithExpirationRequired(), jwt.WithStrictDecoding())

	if err != nil || !token.Valid {
		lg.Debug("invalid token", "path", r.URL.Path, "err", err)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		lg.Debug("invalid token claims type", "path", r.URL.Path)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	policyRule, ok := claims["policyRule"].(string)
	if !ok {
		lg.Debug("policyRule claim is not a string")
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	if policyRule != rule.Hash() {
		lg.Debug("user originally passed with a different rule, issuing new challenge", "old", policyRule, "new", rule.Name)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.RenderIndex(w, r, rule, httpStatusOnly)
		return
	}

	r.Header.Add("X-Anubis-Status", "PASS")
	s.ServeHTTPNext(w, r)
}

func (s *Server) checkRules(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, lg *slog.Logger, rule *policy.Bot) bool {
	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("allowing traffic to origin (explicit)")
		s.ServeHTTPNext(w, r)
		return true
	case config.RuleDeny:
		s.ClearCookie(w, s.cookieName, cookiePath)
		lg.Info("explicit deny")
		if rule == nil {
			lg.Error("rule is nil, cannot calculate checksum")
			s.respondWithError(w, r, "Internal Server Error: Please contact the administrator and ask them to look for the logs around \"maybeReverseProxy.RuleDeny\"")
			return true
		}
		hash := rule.Hash()

		lg.Debug("rule hash", "hash", hash)
		s.respondWithStatus(w, r, fmt.Sprintf("Access Denied: error code %s", hash), s.policy.StatusCodes.Deny)
		return true
	case config.RuleChallenge:
		lg.Debug("challenge requested")
	case config.RuleBenchmark:
		lg.Debug("serving benchmark page")
		s.RenderBench(w, r)
		return true
	default:
		s.ClearCookie(w, s.cookieName, cookiePath)
		slog.Error("CONFIG ERROR: unknown rule", "rule", cr.Rule)
		s.respondWithError(w, r, "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"maybeReverseProxy.Rules\"")
		return true
	}
	return false
}

func (s *Server) handleDNSBL(w http.ResponseWriter, r *http.Request, ip string, lg *slog.Logger) bool {
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
			s.respondWithStatus(w, r, fmt.Sprintf("DroneBL reported an entry: %s, see https://dronebl.org/lookup?ip=%s", resp.String(), ip), s.policy.StatusCodes.Deny)
			return true
		}
	}
	return false
}

func (s *Server) MakeChallenge(w http.ResponseWriter, r *http.Request) {
	lg := internal.GetRequestLogger(r)

	redir := r.FormValue("redir")
	if redir == "" {
		w.WriteHeader(http.StatusBadRequest)
		encoder := json.NewEncoder(w)
		lg.Error("invalid invocation of MakeChallenge", "redir", redir)
		encoder.Encode(struct {
			Error string `json:"error"`
		}{
			Error: "Invalid invocation of MakeChallenge",
		})
		return
	}

	r.URL.Path = redir

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
	chal := s.challengeFor(r, rule.Challenge.Difficulty)

	s.SetCookie(w, anubis.TestCookieName, chal, "/")

	err = encoder.Encode(struct {
		Rules     *config.ChallengeRules `json:"rules"`
		Challenge string                 `json:"challenge"`
	}{
		Challenge: chal,
		Rules:     rule.Challenge,
	})
	if err != nil {
		lg.Error("failed to encode challenge", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lg.Debug("made challenge", "challenge", chal, "rules", rule.Challenge, "cr", cr)
	challengesIssued.WithLabelValues("api").Inc()
}

func (s *Server) PassChallenge(w http.ResponseWriter, r *http.Request) {
	lg := internal.GetRequestLogger(r)

	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	if _, err := r.Cookie(anubis.TestCookieName); errors.Is(err, http.ErrNoCookie) {
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.ClearCookie(w, anubis.TestCookieName, "/")
		lg.Warn("user has cookies disabled, this is not an anubis bug")
		s.respondWithError(w, r, "Your browser is configured to disable cookies. Anubis requires cookies for the legitimate interest of making sure you are a valid client. Please enable cookies for this domain")
		return
	}

	s.ClearCookie(w, anubis.TestCookieName, "/")

	redir := r.FormValue("redir")
	redirURL, err := url.ParseRequestURI(redir)
	if err != nil {
		lg.Error("invalid redirect", "err", err)
		s.respondWithError(w, r, "Invalid redirect")
		return
	}
	// used by the path checker rule
	r.URL = redirURL

	urlParsed, err := r.URL.Parse(redir)
	if err != nil {
		s.respondWithError(w, r, "Redirect URL not parseable")
		return
	}
	if (len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host)) || urlParsed.Host != r.URL.Host {
		s.respondWithError(w, r, "Redirect domain not allowed")
		return
	}

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"passChallenge\"")
		return
	}
	lg = lg.With("check_result", cr)

	impl, ok := challenge.Get(rule.Challenge.Algorithm)
	if !ok {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, fmt.Sprintf("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to file a bug as Anubis is trying to use challenge method %s but it does not exist in the challenge registry", rule.Challenge.Algorithm))
		return
	}

	challengeStr := s.challengeFor(r, rule.Challenge.Difficulty)

	if err := impl.Validate(r, lg, rule, challengeStr); err != nil {
		failedValidations.WithLabelValues(rule.Challenge.Algorithm).Inc()
		var cerr *challenge.Error
		s.ClearCookie(w, s.cookieName, cookiePath)
		lg.Debug("challenge validate call failed", "err", err)

		switch {
		case errors.As(err, &cerr):
			switch {
			case errors.Is(err, challenge.ErrFailed):
				s.respondWithStatus(w, r, cerr.PublicReason, cerr.StatusCode)
			case errors.Is(err, challenge.ErrInvalidFormat), errors.Is(err, challenge.ErrMissingField):
				s.respondWithError(w, r, cerr.PublicReason)
			}
		}
	}

	// generate JWT cookie
	tokenString, err := s.signJWT(jwt.MapClaims{
		"challenge":  challengeStr,
		"method":     rule.Challenge.Algorithm,
		"policyRule": rule.Hash(),
		"action":     string(cr.Rule),
	})
	if err != nil {
		lg.Error("failed to sign JWT", "err", err)
		s.ClearCookie(w, s.cookieName, cookiePath)
		s.respondWithError(w, r, "failed to sign JWT")
		return
	}

	s.SetCookie(w, s.cookieName, tokenString, cookiePath)

	challengesValidated.WithLabelValues(rule.Challenge.Algorithm).Inc()
	lg.Debug("challenge passed, redirecting to app")
	http.Redirect(w, r, redir, http.StatusFound)
}

func cr(name string, rule config.Rule, weight int) policy.CheckResult {
	return policy.CheckResult{
		Name:   name,
		Rule:   rule,
		Weight: weight,
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

	weight := 0

	for _, b := range s.policy.Bots {
		match, err := b.Rules.Check(r)
		if err != nil {
			return decaymap.Zilch[policy.CheckResult](), nil, fmt.Errorf("can't run check %s: %w", b.Name, err)
		}

		if match {
			switch b.Action {
			case config.RuleDeny, config.RuleAllow, config.RuleBenchmark, config.RuleChallenge:
				return cr("bot/"+b.Name, b.Action, weight), &b, nil
			case config.RuleWeigh:
				slog.Debug("adjusting weight", "name", b.Name, "delta", b.Weight.Adjust)
				weight += b.Weight.Adjust
			}
		}
	}

	for _, t := range s.policy.Thresholds {
		result, _, err := t.Program.ContextEval(r.Context(), &policy.ThresholdRequest{Weight: weight})
		if err != nil {
			slog.Error("error when evaluating threshold expression", "expression", t.Expression.String(), "err", err)
			continue
		}

		var matches bool

		if val, ok := result.(types.Bool); ok {
			matches = bool(val)
		}

		if matches {
			return cr("threshold/"+t.Name, t.Action, weight), &policy.Bot{
				Challenge: t.Challenge,
				Rules:     &checker.List{},
			}, nil
		}
	}

	return cr("default/allow", config.RuleAllow, weight), &policy.Bot{
		Challenge: &config.ChallengeRules{
			Difficulty: s.policy.DefaultDifficulty,
			ReportAs:   s.policy.DefaultDifficulty,
			Algorithm:  config.DefaultAlgorithm,
		},
		Rules: &checker.List{},
	}, nil
}

func (s *Server) CleanupDecayMap() {
	s.DNSBLCache.Cleanup()
	s.OGTags.Cleanup()
}
