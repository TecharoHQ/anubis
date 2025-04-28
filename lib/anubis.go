package lib

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dnsbl"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/config"
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
	lg := internal.GetRequestLogger(r)

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

	r.Header.Add("X-Anubis-Status", "PASS")
	s.ServeHTTPNext(w, r)
}

func (s *Server) checkRules(w http.ResponseWriter, r *http.Request, cr policy.CheckResult, lg *slog.Logger, rule *policy.Bot) bool {
	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("allowing traffic to origin (explicit)")
		s.ServeHTTPNext(w, r)
		return true
	case config.RuleDeny:
		s.ClearCookie(w)
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
		s.ClearCookie(w)
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
	lg := internal.GetRequestLogger(r)

	redir := r.FormValue("redir")
	redirURL, err := url.ParseRequestURI(redir)
	if err != nil {
		lg.Error("invalid redirect", "err", err)
		s.respondWithError(w, r, "Invalid redirect")
		return
	}
	// used by the path checker rule
	r.URL = redirURL

	cr, rule, err := s.check(r)
	if err != nil {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, "Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to look for the logs around \"passChallenge\".\"")
		return
	}
	lg = lg.With("check_result", cr)

	nonceStr := r.FormValue("nonce")
	if nonceStr == "" {
		s.ClearCookie(w)
		lg.Debug("no nonce")
		s.respondWithError(w, r, "missing nonce")
		return
	}

	elapsedTimeStr := r.FormValue("elapsedTime")
	if elapsedTimeStr == "" {
		s.ClearCookie(w)
		lg.Debug("no elapsedTime")
		s.respondWithError(w, r, "missing elapsedTime")
		return
	}

	elapsedTime, err := strconv.ParseFloat(elapsedTimeStr, 64)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("elapsedTime doesn't parse", "err", err)
		s.respondWithError(w, r, "invalid elapsedTime")
		return
	}

	lg.Info("challenge took", "elapsedTime", elapsedTime)
	timeTaken.Observe(elapsedTime)

	response := r.FormValue("response")
	urlParsed, err := r.URL.Parse(redir)
	if err != nil {
		s.respondWithError(w, r, "Redirect URL not parseable")
		return
	}
	if (len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host)) || urlParsed.Host != r.URL.Host {
		s.respondWithError(w, r, "Redirect domain not allowed")
		return
	}

	challenge := s.challengeFor(r, rule.Challenge.Difficulty)

	nonce, err := strconv.Atoi(nonceStr)
	if err != nil {
		s.ClearCookie(w)
		lg.Debug("nonce doesn't parse", "err", err)
		s.respondWithError(w, r, "invalid nonce")
		return
	}

	calcString := fmt.Sprintf("%s%d", challenge, nonce)
	calculated := internal.SHA256sum(calcString)

	if subtle.ConstantTimeCompare([]byte(response), []byte(calculated)) != 1 {
		s.ClearCookie(w)
		lg.Debug("hash does not match", "got", response, "want", calculated)
		s.respondWithStatus(w, r, "invalid response", http.StatusForbidden)
		failedValidations.Inc()
		return
	}

	// compare the leading zeroes
	if !strings.HasPrefix(response, strings.Repeat("0", rule.Challenge.Difficulty)) {
		s.ClearCookie(w)
		lg.Debug("difficulty check failed", "response", response, "difficulty", rule.Challenge.Difficulty)
		s.respondWithStatus(w, r, "invalid response", http.StatusForbidden)
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
		"exp":       time.Now().Add(s.opts.CookieExpiration).Unix(),
	})
	tokenString, err := token.SignedString(s.priv)
	if err != nil {
		lg.Error("failed to sign JWT", "err", err)
		s.ClearCookie(w)
		s.respondWithError(w, r, "failed to sign JWT")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:        anubis.CookieName,
		Value:       tokenString,
		Expires:     time.Now().Add(s.opts.CookieExpiration),
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
	s.respondWithError(w, r, err)
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
