package lib

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/cel-go/common/types"
	"github.com/google/uuid"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/dns"
	"github.com/TecharoHQ/anubis/internal/dnsbl"
	"github.com/TecharoHQ/anubis/internal/ogtags"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/TecharoHQ/anubis/lib/store"

	// challenge implementations
	_ "github.com/TecharoHQ/anubis/lib/challenge/metarefresh"
	_ "github.com/TecharoHQ/anubis/lib/challenge/preact"
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
	next        http.Handler
	store       store.Interface
	mux         *http.ServeMux
	policy      *policy.ParsedConfig
	OGTags      *ogtags.OGTagCache
	dnsCache    *dns.DnsCache
	logger      *slog.Logger
	opts        Options
	ed25519Priv ed25519.PrivateKey
	hs512Secret []byte
}

func (s *Server) getTokenKeyfunc() jwt.Keyfunc {
	// return ED25519 key if HS512 is not set
	if len(s.hs512Secret) == 0 {
		return func(token *jwt.Token) (interface{}, error) {
			return s.ed25519Priv.Public().(ed25519.PublicKey), nil
		}
	} else {
		return func(token *jwt.Token) (interface{}, error) {
			return s.hs512Secret, nil
		}
	}
}

func (s *Server) getChallenge(r *http.Request) (*challenge.Challenge, error) {
	id := r.FormValue("id")
	j := store.JSON[challenge.Challenge]{Underlying: s.store}

	chall, err := j.Get(r.Context(), "challenge:"+id)

	return &chall, err
}

func (s *Server) issueChallenge(ctx context.Context, r *http.Request, lg *slog.Logger, cr policy.CheckResult, rule *policy.Bot) (*challenge.Challenge, error) {
	if cr.Rule != config.RuleChallenge {
		slog.Error("this should be impossible, asked to issue a challenge but the rule is not a challenge rule", "cr", cr, "rule", rule)
		//return nil, errors.New("[unexpected] this codepath should be impossible, asked to issue a challenge for a non-challenge rule")
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	var randomData = make([]byte, 64)
	if _, err := rand.Read(randomData); err != nil {
		return nil, err
	}

	chall := challenge.Challenge{
		ID:             id.String(),
		Method:         rule.Challenge.Algorithm,
		RandomData:     fmt.Sprintf("%x", randomData),
		IssuedAt:       time.Now(),
		Difficulty:     rule.Challenge.Difficulty,
		PolicyRuleHash: rule.Hash(),
		Metadata: map[string]string{
			"User-Agent": r.Header.Get("User-Agent"),
			"X-Real-Ip":  r.Header.Get("X-Real-Ip"),
		},
	}

	j := store.JSON[challenge.Challenge]{Underlying: s.store}
	if err := j.Set(ctx, "challenge:"+id.String(), chall, 30*time.Minute); err != nil {
		return nil, err
	}

	lg.Info("new challenge issued", "challenge", id.String())

	return &chall, err
}

func (s *Server) hydrateChallengeRule(rule *policy.Bot, chall *challenge.Challenge, lg *slog.Logger) *policy.Bot {
	if chall == nil {
		return rule
	}

	if rule == nil {
		rule = &policy.Bot{
			Rules: &checker.List{},
		}
	}

	if chall.Difficulty == 0 {
		// fall back to whatever the policy currently says or the global default
		if rule.Challenge != nil && rule.Challenge.Difficulty != 0 {
			chall.Difficulty = rule.Challenge.Difficulty
		} else {
			chall.Difficulty = s.policy.DefaultDifficulty
		}
	}

	if rule.Challenge == nil {
		lg.Warn("rule missing challenge configuration; using stored challenge metadata", "rule", rule.Name)
		rule.Challenge = &config.ChallengeRules{}
	}

	if rule.Challenge.Difficulty == 0 {
		rule.Challenge.Difficulty = chall.Difficulty
	}
	if rule.Challenge.ReportAs == 0 {
		rule.Challenge.ReportAs = chall.Difficulty
	}
	if rule.Challenge.Algorithm == "" {
		rule.Challenge.Algorithm = chall.Method
	}

	return rule
}

func (s *Server) maybeReverseProxyHttpStatusOnly(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, true)
}

func (s *Server) maybeReverseProxyOrPage(w http.ResponseWriter, r *http.Request) {
	s.maybeReverseProxy(w, r, false)
}

func (s *Server) maybeReverseProxy(w http.ResponseWriter, r *http.Request, httpStatusOnly bool) {
	lg := internal.GetRequestLogger(s.logger, r)

	if val, _ := s.store.Get(r.Context(), fmt.Sprintf("ogtags:allow:%s%s", r.Host, r.URL.String())); val != nil {
		lg.Debug("serving opengraph tag asset")
		s.ServeHTTPNext(w, r)
		return
	}

	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	cr, rule, err := s.check(r, lg)
	if err != nil {
		lg.Error("check failed", "err", err)
		localizer := localization.GetLocalizer(r)
		s.respondWithError(w, r, fmt.Sprintf("%s \"maybeReverseProxy\"", localizer.T("internal_server_error")), makeCode(err))
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
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	if err := ckie.Valid(); err != nil {
		lg.Debug("cookie is invalid", "err", err)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	if time.Now().After(ckie.Expires) && !ckie.Expires.IsZero() {
		lg.Debug("cookie expired", "path", r.URL.Path)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	token, err := jwt.ParseWithClaims(ckie.Value, jwt.MapClaims{}, s.getTokenKeyfunc(), jwt.WithExpirationRequired(), jwt.WithStrictDecoding())

	if err != nil || !token.Valid {
		lg.Debug("invalid token", "path", r.URL.Path, "err", err)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		lg.Debug("invalid token claims type", "path", r.URL.Path)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	policyRule, ok := claims["policyRule"].(string)
	if !ok {
		lg.Debug("policyRule claim is not a string")
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	if policyRule != rule.Hash() {
		lg.Debug("user originally passed with a different rule, issuing new challenge", "old", policyRule, "new", rule.Name)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
		return
	}

	if s.opts.JWTRestrictionHeader != "" && claims["restriction"] != internal.SHA256sum(r.Header.Get(s.opts.JWTRestrictionHeader)) {
		lg.Debug("JWT restriction header is invalid")
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.RenderIndex(w, r, cr, rule, httpStatusOnly)
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

	localizer := localization.GetLocalizer(r)

	switch cr.Rule {
	case config.RuleAllow:
		lg.Debug("allowing traffic to origin (explicit)")
		s.ServeHTTPNext(w, r)
		return true
	case config.RuleDeny:
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		lg.Info("explicit deny")
		if rule == nil {
			lg.Error("rule is nil, cannot calculate checksum")
			s.respondWithError(w, r, fmt.Sprintf("%s \"maybeReverseProxy.RuleDeny\"", localizer.T("internal_server_error")), makeCode(ErrActualAnubisBug))
			return true
		}
		hash := rule.Hash()

		lg.Debug("rule hash", "hash", hash)
		s.respondWithStatus(w, r, fmt.Sprintf("%s %s", localizer.T("access_denied"), hash), "", s.policy.StatusCodes.Deny)
		return true
	case config.RuleChallenge:
		lg.Debug("challenge requested")
	case config.RuleBenchmark:
		lg.Debug("serving benchmark page")
		s.RenderBench(w, r)
		return true
	default:
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		lg.Error("CONFIG ERROR: unknown rule", "rule", cr.Rule)
		s.respondWithError(w, r, fmt.Sprintf("%s \"maybeReverseProxy.Rules\"", localizer.T("internal_server_error")), makeCode(ErrActualAnubisBug))
		return true
	}
	return false
}

func (s *Server) handleDNSBL(w http.ResponseWriter, r *http.Request, ip string, lg *slog.Logger) bool {
	db := &store.JSON[dnsbl.DroneBLResponse]{Underlying: s.store, Prefix: "dronebl:"}
	if s.policy.DNSBL && ip != "" {
		resp, err := db.Get(r.Context(), ip)
		if err != nil {
			lg.Debug("looking up ip in dnsbl")
			resp, err := dnsbl.Lookup(ip)
			if err != nil {
				lg.Error("can't look up ip in dnsbl", "err", err)
			}
			db.Set(r.Context(), ip, resp, 24*time.Hour)
			droneBLHits.WithLabelValues(resp.String()).Inc()
		}

		if resp != dnsbl.AllGood {
			lg.Info("DNSBL hit", "status", resp.String())
			localizer := localization.GetLocalizer(r)
			s.respondWithStatus(w, r, fmt.Sprintf("%s: %s, %s https://dronebl.org/lookup?ip=%s",
				localizer.T("dronebl_entry"),
				resp.String(),
				localizer.T("see_dronebl_lookup"),
				ip), "", s.policy.StatusCodes.Deny)
			return true
		}
	}
	return false
}

func (s *Server) MakeChallenge(w http.ResponseWriter, r *http.Request) {
	lg := internal.GetRequestLogger(s.logger, r)
	localizer := localization.GetLocalizer(r)

	redir := r.FormValue("redir")
	if redir == "" {
		w.WriteHeader(http.StatusBadRequest)
		encoder := json.NewEncoder(w)
		lg.Error("invalid invocation of MakeChallenge", "redir", redir)
		encoder.Encode(struct {
			Error string `json:"error"`
		}{
			Error: localizer.T("invalid_invocation"),
		})
		return
	}

	r.URL.Path = redir

	encoder := json.NewEncoder(w)
	cr, rule, err := s.check(r, lg)
	if err != nil {
		lg.Error("check failed", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		err := encoder.Encode(struct {
			Error string `json:"error"`
		}{
			Error: fmt.Sprintf("%s \"makeChallenge\"", localizer.T("internal_server_error")),
		})
		if err != nil {
			lg.Error("failed to encode error response", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	lg = lg.With("check_result", cr)

	chall, err := s.issueChallenge(r.Context(), r, lg, cr, rule)
	if err != nil {
		lg.Error("failed to fetch or issue challenge", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		err := encoder.Encode(struct {
			Error string `json:"error"`
		}{
			Error: fmt.Sprintf("%s \"makeChallenge\"", localizer.T("internal_server_error")),
		})
		if err != nil {
			lg.Error("failed to encode error response", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	s.SetCookie(w, CookieOpts{Host: r.Host, Name: anubis.TestCookieName, Value: chall.ID})

	err = encoder.Encode(struct {
		Rules     *config.ChallengeRules `json:"rules"`
		Challenge string                 `json:"challenge"`
		ID        string                 `json:"id"`
	}{
		Rules:     rule.Challenge,
		Challenge: chall.RandomData,
		ID:        chall.ID,
	})
	if err != nil {
		lg.Error("failed to encode challenge", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	lg.Debug("made challenge", "challenge", chall, "rules", rule.Challenge, "cr", cr)
	challengesIssued.WithLabelValues("api").Inc()
}

func (s *Server) PassChallenge(w http.ResponseWriter, r *http.Request) {
	lg := internal.GetRequestLogger(s.logger, r)
	localizer := localization.GetLocalizer(r)

	redir := r.FormValue("redir")
	redirURL, err := url.ParseRequestURI(redir)
	if err != nil {
		lg.Error("invalid redirect", "err", err)
		s.respondWithStatus(w, r, localizer.T("invalid_redirect"), makeCode(err), http.StatusBadRequest)
		return
	}

	switch redirURL.Scheme {
	case "", "http", "https":
		// allowed
	default:
		lg.Error("XSS attempt blocked, invalid redirect scheme", "scheme", redirURL.Scheme)
		s.respondWithStatus(w, r, localizer.T("invalid_redirect"), "", http.StatusBadRequest)
		return
	}

	// Adjust cookie path if base prefix is not empty
	cookiePath := "/"
	if anubis.BasePrefix != "" {
		cookiePath = strings.TrimSuffix(anubis.BasePrefix, "/") + "/"
	}

	if _, err := r.Cookie(anubis.TestCookieName); errors.Is(err, http.ErrNoCookie) {
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.ClearCookie(w, CookieOpts{Name: anubis.TestCookieName, Host: r.Host})
		lg.Warn("user has cookies disabled, this is not an anubis bug")
		s.respondWithError(w, r, localizer.T("cookies_disabled"), "")
		return
	}

	// used by the path checker rule
	r.URL = redirURL

	urlParsed, err := r.URL.Parse(redir)
	if err != nil {
		s.respondWithError(w, r, localizer.T("redirect_not_parseable"), makeCode(err))
		return
	}
	if (len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !matchRedirectDomain(s.opts.RedirectDomains, urlParsed.Host)) || urlParsed.Host != r.URL.Host {
		lg.Debug("domain not allowed", "domain", urlParsed.Host)
		s.respondWithError(w, r, localizer.T("redirect_domain_not_allowed"), "")
		return
	}

	cr, rule, err := s.check(r, lg)
	if err != nil {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, fmt.Sprintf("%s \"passChallenge\"", localizer.T("internal_server_error")), makeCode(err))
		return
	}
	lg = lg.With("check_result", cr)

	chall, err := s.getChallenge(r)
	if err != nil {
		lg.Error("getChallenge failed", "err", err)
		s.respondWithError(w, r, fmt.Sprintf("%s: %s", localizer.T("internal_server_error"), rule.Challenge.Algorithm), makeCode(err))
		return
	}

	if chall.Spent {
		lg.Error("double spend prevented", "reason", "double_spend")
		s.respondWithError(w, r, fmt.Sprintf("%s: %s", localizer.T("internal_server_error"), "double_spend"), "")
		return
	}

	rule = s.hydrateChallengeRule(rule, chall, lg)

	impl, ok := challenge.Get(chall.Method)
	if !ok {
		lg.Error("check failed", "err", err)
		s.respondWithError(w, r, fmt.Sprintf("%s: %s", localizer.T("internal_server_error"), rule.Challenge.Algorithm), makeCode(ErrActualAnubisBug))
		return
	}

	lg = lg.With("challenge", chall.ID)

	in := &challenge.ValidateInput{
		Challenge: chall,
		Rule:      rule,
		Store:     s.store,
	}

	if err := impl.Validate(r, lg, in); err != nil {
		failedValidations.WithLabelValues(rule.Challenge.Algorithm).Inc()
		var cerr *challenge.Error
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		lg.Debug("challenge validate call failed", "err", err)

		switch {
		case errors.As(err, &cerr):
			switch {
			case errors.Is(err, challenge.ErrFailed):
				lg.Error("challenge failed", "err", err)
				s.respondWithStatus(w, r, cerr.PublicReason, makeCode(err), cerr.StatusCode)
				return
			case errors.Is(err, challenge.ErrInvalidFormat), errors.Is(err, challenge.ErrMissingField):
				lg.Error("invalid challenge format", "err", err)
				s.respondWithError(w, r, cerr.PublicReason, makeCode(err))
				return
			}
		}
	}

	// generate JWT cookie
	var tokenString string

	// check if JWTRestrictionHeader is set and header is in request
	claims := jwt.MapClaims{
		"challenge":  chall.ID,
		"method":     rule.Challenge.Algorithm,
		"policyRule": rule.Hash(),
		"action":     string(cr.Rule),
	}
	if s.opts.JWTRestrictionHeader != "" {
		if r.Header.Get(s.opts.JWTRestrictionHeader) == "" {
			lg.Error("JWTRestrictionHeader is set in config but not found in request, please check your reverse proxy config.")
			s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
			s.respondWithError(w, r, "failed to sign JWT", makeCode(err))
			return
		} else {
			claims["restriction"] = internal.SHA256sum(r.Header.Get(s.opts.JWTRestrictionHeader))
		}
	}
	if s.opts.DifficultyInJWT {
		claims["difficulty"] = rule.Challenge.Difficulty
	}
	tokenString, err = s.signJWT(claims)

	if err != nil {
		lg.Error("failed to sign JWT", "err", err)
		s.ClearCookie(w, CookieOpts{Path: cookiePath, Host: r.Host})
		s.respondWithError(w, r, localizer.T("failed_to_sign_jwt"), makeCode(err))
		return
	}

	s.SetCookie(w, CookieOpts{Path: cookiePath, Host: r.Host, Value: tokenString})

	chall.Spent = true
	j := store.JSON[challenge.Challenge]{Underlying: s.store}
	if err := j.Set(r.Context(), "challenge:"+chall.ID, *chall, 30*time.Minute); err != nil {
		lg.Debug("can't update information about challenge", "err", err)
	}

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
func (s *Server) check(r *http.Request, lg *slog.Logger) (policy.CheckResult, *policy.Bot, error) {
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
				lg.Debug("adjusting weight", "name", b.Name, "delta", b.Weight.Adjust)
				policy.Applications.WithLabelValues("bot/"+b.Name, "WEIGH").Add(1)
				weight += b.Weight.Adjust
			}
		}
	}

	for _, t := range s.policy.Thresholds {
		result, _, err := t.Program.ContextEval(r.Context(), &policy.ThresholdRequest{Weight: weight})
		if err != nil {
			lg.Error("error when evaluating threshold expression", "expression", t.Expression.String(), "err", err)
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
