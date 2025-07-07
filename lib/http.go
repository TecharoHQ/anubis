package lib

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/a-h/templ"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/net/publicsuffix"
)

var domainMatchRegexp = regexp.MustCompile(`^((xn--)?[a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$`)

type CookieOpts struct {
	Value  string
	Host   string
	Path   string
	Name   string
	Expiry time.Duration
}

func (s *Server) SetCookie(w http.ResponseWriter, cookieOpts CookieOpts) {
	var domain = s.opts.CookieDomain
	var name = anubis.CookieName
	var path = "/"
	if cookieOpts.Name != "" {
		name = cookieOpts.Name
	}
	if cookieOpts.Path != "" {
		path = cookieOpts.Path
	}
	if s.opts.CookieDynamicDomain && domainMatchRegexp.MatchString(cookieOpts.Host) {
		if etld, err := publicsuffix.EffectiveTLDPlusOne(cookieOpts.Host); err == nil {
			domain = etld
		}
	}

	if cookieOpts.Expiry == 0 {
		cookieOpts.Expiry = s.opts.CookieExpiration
	}

	http.SetCookie(w, &http.Cookie{
		Name:        name,
		Value:       cookieOpts.Value,
		Expires:     time.Now().Add(cookieOpts.Expiry),
		SameSite:    http.SameSiteNoneMode,
		Domain:      domain,
		Secure:      s.opts.CookieSecure,
		Partitioned: s.opts.CookiePartitioned,
		Path:        path,
	})
}

func (s *Server) ClearCookie(w http.ResponseWriter, cookieOpts CookieOpts) {
	var domain = s.opts.CookieDomain
	var name = anubis.CookieName
	var path = "/"
	if cookieOpts.Name != "" {
		name = cookieOpts.Name
	}
	if cookieOpts.Path != "" {
		path = cookieOpts.Path
	}
	if s.opts.CookieDynamicDomain && domainMatchRegexp.MatchString(cookieOpts.Host) {
		if etld, err := publicsuffix.EffectiveTLDPlusOne(cookieOpts.Host); err == nil {
			domain = etld
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:        name,
		Value:       "",
		MaxAge:      -1,
		Expires:     time.Now().Add(-1 * time.Minute),
		SameSite:    http.SameSiteNoneMode,
		Partitioned: s.opts.CookiePartitioned,
		Domain:      domain,
		Secure:      s.opts.CookieSecure,
		Path:        path,
	})
}

// https://github.com/oauth2-proxy/oauth2-proxy/blob/master/pkg/upstream/http.go#L124
type UnixRoundTripper struct {
	Transport *http.Transport
}

// set bare minimum stuff
func (t UnixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if req.Host == "" {
		req.Host = "localhost"
	}
	req.URL.Host = req.Host // proxy error: no Host in request URL
	req.URL.Scheme = "http" // make http.Transport happy and avoid an infinite recursion
	return t.Transport.RoundTrip(req)
}

func randomChance(n int) bool {
	return rand.Intn(n) == 0
}

func (s *Server) RenderIndex(w http.ResponseWriter, r *http.Request, rule *policy.Bot, returnHTTPStatusOnly bool) {
	localizer := localization.GetLocalizer(r)

	if returnHTTPStatusOnly {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(localizer.T("authorization_required")))
		return
	}

	lg := internal.GetRequestLogger(r)

	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && randomChance(64) {
		lg.Error("client was given a challenge but does not in fact support gzip compression")
		s.respondWithError(w, r, localizer.T("client_error_browser"))
	}

	challengesIssued.WithLabelValues("embedded").Add(1)
	chall, err := s.challengeFor(r)
	if err != nil {
		lg.Error("can't get challenge", "err", "err")
		s.respondWithError(w, r, fmt.Sprintf("%s: %s", localizer.T("internal_server_error"), rule.Challenge.Algorithm))
		return
	}

	var ogTags map[string]string = nil
	if s.opts.OpenGraph.Enabled {
		var err error
		ogTags, err = s.OGTags.GetOGTags(r.Context(), r.URL, r.Host)
		if err != nil {
			lg.Error("failed to get OG tags", "err", err)
		}
	}

	s.SetCookie(w, CookieOpts{
		Value:  chall.ID,
		Host:   r.Host,
		Path:   "/",
		Name:   anubis.TestCookieName,
		Expiry: 30 * time.Minute,
	})

	impl, ok := challenge.Get(rule.Challenge.Algorithm)
	if !ok {
		lg.Error("check failed", "err", "can't get algorithm", "algorithm", rule.Challenge.Algorithm)
		s.respondWithError(w, r, fmt.Sprintf("%s: %s", localizer.T("internal_server_error"), rule.Challenge.Algorithm))
		return
	}

	in := &challenge.IssueInput{
		Impressum: s.policy.Impressum,
		Rule:      rule,
		Challenge: chall,
		OGTags:    ogTags,
		Store:     s.store,
	}

	component, err := impl.Issue(r, lg, in)
	if err != nil {
		lg.Error("[unexpected] render failed, please open an issue", "err", err) // This is likely a bug in the template. Should never be triggered as CI tests for this.
		s.respondWithError(w, r, fmt.Sprintf("%s \"RenderIndex\"", localizer.T("internal_server_error")))
		return
	}

	handler := internal.GzipMiddleware(1, internal.NoStoreCache(templ.Handler(
		component,
		templ.WithStatus(s.opts.Policy.StatusCodes.Challenge),
	)))
	handler.ServeHTTP(w, r)
}

func (s *Server) RenderBench(w http.ResponseWriter, r *http.Request) {
	localizer := localization.GetLocalizer(r)

	templ.Handler(
		web.Base(localizer.T("benchmarking_anubis"), web.Bench(localizer), s.policy.Impressum, localizer),
	).ServeHTTP(w, r)
}

func (s *Server) respondWithError(w http.ResponseWriter, r *http.Request, message string) {
	s.respondWithStatus(w, r, message, http.StatusInternalServerError)
}

func (s *Server) respondWithStatus(w http.ResponseWriter, r *http.Request, msg string, status int) {
	localizer := localization.GetLocalizer(r)

	templ.Handler(web.Base(localizer.T("oh_noes"), web.ErrorPage(msg, s.opts.WebmasterEmail, localizer), s.policy.Impressum, localizer), templ.WithStatus(status)).ServeHTTP(w, r)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) stripBasePrefixFromRequest(r *http.Request) *http.Request {
	if !s.opts.StripBasePrefix || s.opts.BasePrefix == "" {
		return r
	}

	basePrefix := strings.TrimSuffix(s.opts.BasePrefix, "/")
	path := r.URL.Path

	if !strings.HasPrefix(path, basePrefix) {
		return r
	}

	trimmedPath := strings.TrimPrefix(path, basePrefix)
	if trimmedPath == "" {
		trimmedPath = "/"
	}

	// Clone the request and URL
	reqCopy := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = trimmedPath
	reqCopy.URL = &urlCopy

	return reqCopy
}

func (s *Server) ServeHTTPNext(w http.ResponseWriter, r *http.Request) {
	if s.next == nil {
		localizer := localization.GetLocalizer(r)

		redir := r.FormValue("redir")
		urlParsed, err := r.URL.Parse(redir)
		if err != nil {
			s.respondWithStatus(w, r, localizer.T("redirect_not_parseable"), http.StatusBadRequest)
			return
		}

		if (len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host)) || urlParsed.Host != r.URL.Host {
			s.respondWithStatus(w, r, localizer.T("redirect_domain_not_allowed"), http.StatusBadRequest)
			return
		}

		if redir != "" {
			http.Redirect(w, r, redir, http.StatusFound)
			return
		}

		templ.Handler(
			web.Base(localizer.T("you_are_not_a_bot"), web.StaticHappy(localizer), s.policy.Impressum, localizer),
		).ServeHTTP(w, r)
	} else {
		requestsProxied.WithLabelValues(r.Host).Inc()
		r = s.stripBasePrefixFromRequest(r)
		s.next.ServeHTTP(w, r)
	}
}

func (s *Server) signJWT(claims jwt.MapClaims) (string, error) {
	claims["iat"] = time.Now().Unix()
	claims["nbf"] = time.Now().Add(-1 * time.Minute).Unix()
	claims["exp"] = time.Now().Add(s.opts.CookieExpiration).Unix()

	if len(s.hs512Secret) == 0 {
		return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(s.ed25519Priv)
	} else {
		return jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(s.hs512Secret)
	}
}
