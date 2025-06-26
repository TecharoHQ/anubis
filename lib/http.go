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
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/a-h/templ"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/net/publicsuffix"
)

var domainMatchRegexp = regexp.MustCompile(`^((xn--)?[a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$`)

func (s *Server) SetCookie(w http.ResponseWriter, name, value, path, host string) {
	var domain = s.opts.CookieDomain
	if s.opts.CookieDynamicDomain && domainMatchRegexp.MatchString(host) {
		if etld, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
			domain = etld
			name = anubis.WithDomainCookieName + etld
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:        name,
		Value:       value,
		Expires:     time.Now().Add(s.opts.CookieExpiration),
		SameSite:    http.SameSiteLaxMode,
		Domain:      domain,
		Partitioned: s.opts.CookiePartitioned,
		Path:        path,
	})
}

func (s *Server) ClearCookie(w http.ResponseWriter, name, path, host string) {
	var domain = s.opts.CookieDomain
	if s.opts.CookieDynamicDomain && domainMatchRegexp.MatchString(host) {
		if etld, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
			domain = etld
			name = anubis.WithDomainCookieName + etld
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:        name,
		Value:       "",
		MaxAge:      -1,
		Expires:     time.Now().Add(-1 * time.Minute),
		SameSite:    http.SameSiteLaxMode,
		Partitioned: s.opts.CookiePartitioned,
		Domain:      domain,
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
	if returnHTTPStatusOnly {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Authorization required"))
		return
	}

	lg := internal.GetRequestLogger(r)

	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && randomChance(64) {
		lg.Error("client was given a challenge but does not in fact support gzip compression")
		s.respondWithError(w, r, "Client Error: Please ensure your browser is up to date and try again later.")
	}

	challengesIssued.WithLabelValues("embedded").Add(1)
	challengeStr := s.challengeFor(r, rule.Challenge.Difficulty)

	var ogTags map[string]string = nil
	if s.opts.OpenGraph.Enabled {
		var err error
		ogTags, err = s.OGTags.GetOGTags(r.URL, r.Host)
		if err != nil {
			lg.Error("failed to get OG tags", "err", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:    anubis.TestCookieName,
		Value:   challengeStr,
		Expires: time.Now().Add(30 * time.Minute),
		Path:    "/",
	})

	impl, ok := challenge.Get(rule.Challenge.Algorithm)
	if !ok {
		lg.Error("check failed", "err", "can't get algorithm", "algorithm", rule.Challenge.Algorithm)
		s.respondWithError(w, r, fmt.Sprintf("Internal Server Error: administrator has misconfigured Anubis. Please contact the administrator and ask them to file a bug as Anubis is trying to use challenge method %s but it does not exist in the challenge registry", rule.Challenge.Algorithm))
		return
	}

	in := &challenge.IssueInput{
		Impressum: s.policy.Impressum,
		Rule:      rule,
		Challenge: challengeStr,
		OGTags:    ogTags,
	}

	component, err := impl.Issue(r, lg, in)
	if err != nil {
		lg.Error("[unexpected] render failed, please open an issue", "err", err) // This is likely a bug in the template. Should never be triggered as CI tests for this.
		s.respondWithError(w, r, "Internal Server Error: please contact the administrator and ask them to look for the logs around \"RenderIndex\"")
		return
	}

	handler := internal.GzipMiddleware(1, internal.NoStoreCache(templ.Handler(
		component,
		templ.WithStatus(s.opts.Policy.StatusCodes.Challenge),
	)))
	handler.ServeHTTP(w, r)
}

func (s *Server) RenderBench(w http.ResponseWriter, r *http.Request) {
	templ.Handler(
		web.Base("Benchmarking Anubis!", web.Bench(), s.policy.Impressum),
	).ServeHTTP(w, r)
}

func (s *Server) respondWithError(w http.ResponseWriter, r *http.Request, message string) {
	s.respondWithStatus(w, r, message, http.StatusInternalServerError)
}

func (s *Server) respondWithStatus(w http.ResponseWriter, r *http.Request, msg string, status int) {
	templ.Handler(web.Base("Oh noes!", web.ErrorPage(msg, s.opts.WebmasterEmail), s.policy.Impressum), templ.WithStatus(status)).ServeHTTP(w, r)
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
		redir := r.FormValue("redir")
		urlParsed, err := r.URL.Parse(redir)
		if err != nil {
			s.respondWithStatus(w, r, "Redirect URL not parseable", http.StatusBadRequest)
			return
		}

		if (len(urlParsed.Host) > 0 && len(s.opts.RedirectDomains) != 0 && !slices.Contains(s.opts.RedirectDomains, urlParsed.Host)) || urlParsed.Host != r.URL.Host {
			s.respondWithStatus(w, r, "Redirect domain not allowed", http.StatusBadRequest)
			return
		}

		if redir != "" {
			http.Redirect(w, r, redir, http.StatusFound)
			return
		}

		templ.Handler(
			web.Base("You are not a bot!", web.StaticHappy(), s.policy.Impressum),
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
