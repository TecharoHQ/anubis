package entrypoint

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/TecharoHQ/anubis/cmd/osiris/internal/config"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/fingerprint"
	"github.com/felixge/httpsnoop"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/lum8rjack/go-ja4h"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ErrTargetInvalid     = errors.New("[unexpected] target invalid")
	ErrNoHandler         = errors.New("[unexpected] no handler for domain")
	ErrInvalidTLSKeypair = errors.New("[unexpected] invalid TLS keypair")
	ErrNoCert            = errors.New("this server does not have a certificate for that domain")

	requestsPerDomain = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "techaro",
		Subsystem: "osiris",
		Name:      "request_count",
	}, []string{"domain", "method", "response_code"})

	responseTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "techaro",
		Subsystem: "osiris",
		Name:      "response_time",
	}, []string{"domain"})

	unresolvedRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "techaro",
		Subsystem: "osiris",
		Name:      "unresolved_requests",
	})
)

type Router struct {
	lock     sync.RWMutex
	routes   map[string]http.Handler
	tlsCerts map[string]*tls.Certificate
	opts     Options
}

func (rtr *Router) setConfig(c config.Toplevel) error {
	var errs []error
	newMap := map[string]http.Handler{}
	newCerts := map[string]*tls.Certificate{}

	for _, d := range c.Domains {
		var domainErrs []error

		u, err := url.Parse(d.Target)
		if err != nil {
			domainErrs = append(domainErrs, fmt.Errorf("%w %q: %v", ErrTargetInvalid, d.Target, err))
		}

		var h http.Handler

		if u != nil {
			switch u.Scheme {
			case "http", "https":
				rp := httputil.NewSingleHostReverseProxy(u)

				if d.InsecureSkipVerify {
					rp.Transport = &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: true,
						},
					}
				}

				h = rp
			case "h2c":
				h = newH2CReverseProxy(u)
			case "unix":
				h = &httputil.ReverseProxy{
					Director: func(r *http.Request) {
						r.URL.Scheme = "http"
						r.URL.Host = d.Name
						r.Host = d.Name
					},
					Transport: &http.Transport{
						DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
							return net.Dial("unix", strings.TrimPrefix(d.Target, "unix://"))
						},
					},
				}
			}
		}

		if h == nil {
			domainErrs = append(domainErrs, ErrNoHandler)
		}

		newMap[d.Name] = h

		cert, err := tls.LoadX509KeyPair(d.TLS.Cert, d.TLS.Key)
		if err != nil {
			domainErrs = append(domainErrs, fmt.Errorf("%w: %w", ErrInvalidTLSKeypair, err))
		}

		newCerts[d.Name] = &cert

		if len(domainErrs) != 0 {
			errs = append(errs, fmt.Errorf("invalid domain %s: %w", d.Name, errors.Join(domainErrs...)))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("can't compile config to routing map: %w", errors.Join(errs...))
	}

	rtr.lock.Lock()
	rtr.routes = newMap
	rtr.tlsCerts = newCerts
	rtr.lock.Unlock()

	return nil
}

func (rtr *Router) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	rtr.lock.RLock()
	cert, ok := rtr.tlsCerts[hello.ServerName]
	rtr.lock.RUnlock()

	if !ok {
		return nil, ErrNoCert
	}

	return cert, nil
}

func (rtr *Router) loadConfig() error {
	slog.Info("reloading config", "fname", rtr.opts.ConfigFname)
	var cfg config.Toplevel
	if err := hclsimple.DecodeFile(rtr.opts.ConfigFname, nil, &cfg); err != nil {
		return err
	}

	if err := cfg.Valid(); err != nil {
		return err
	}

	if err := rtr.setConfig(cfg); err != nil {
		return err
	}

	slog.Info("done!")

	return nil
}

func (rtr *Router) backgroundReloadConfig(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := rtr.loadConfig(); err != nil {
				slog.Error("can't reload config", "fname", rtr.opts.ConfigFname, "err", err)
			}
		case <-ch:
			if err := rtr.loadConfig(); err != nil {
				slog.Error("can't reload config", "fname", rtr.opts.ConfigFname, "err", err)
			}
		}
	}
}

func NewRouter(c config.Toplevel) (*Router, error) {
	result := &Router{
		routes: map[string]http.Handler{},
	}

	if err := result.setConfig(c); err != nil {
		return nil, err
	}

	return result, nil
}

func (rtr *Router) HandleHTTP(ctx context.Context, ln net.Listener) error {
	srv := http.Server{
		Handler:  rtr,
		ErrorLog: internal.GetFilteredHTTPLogger(),
	}

	go func(ctx context.Context) {
		<-ctx.Done()
		srv.Close()
	}(ctx)

	return srv.Serve(ln)
}

func (rtr *Router) HandleHTTPS(ctx context.Context, ln net.Listener) error {
	tc := &tls.Config{
		GetCertificate: rtr.GetCertificate,
	}

	srv := &http.Server{
		Handler:   rtr,
		ErrorLog:  internal.GetFilteredHTTPLogger(),
		TLSConfig: tc,
	}

	go func(ctx context.Context) {
		<-ctx.Done()
		srv.Close()
	}(ctx)

	fingerprint.ApplyTLSFingerprinter(srv)

	return srv.ServeTLS(ln, "", "")
}

func (rtr *Router) ListenAndServeMetrics(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("(metrics) can't bind to tcp %s: %w", addr, err)
	}
	defer ln.Close()

	go func(ctx context.Context) {
		<-ctx.Done()
		ln.Close()
	}(ctx)

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/readyz", readyz)
	mux.HandleFunc("/healthz", healthz)

	slog.Info("listening", "for", "metrics", "bind", addr)

	srv := http.Server{
		Addr:     addr,
		Handler:  mux,
		ErrorLog: internal.GetFilteredHTTPLogger(),
	}

	go func(ctx context.Context) {
		<-ctx.Done()
		srv.Close()
	}(ctx)

	return srv.Serve(ln)
}

func (rtr *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var host = r.Host

	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}

	var h http.Handler
	var ok bool

	ja4hFP := ja4h.JA4H(r)

	slog.Info("got request", "method", r.Method, "host", host, "path", r.URL.Path)

	rtr.lock.RLock()
	h, ok = rtr.routes[host]
	rtr.lock.RUnlock()

	if !ok {
		unresolvedRequests.Inc()
		http.NotFound(w, r) // TODO(Xe): brand this
		return
	}

	r.Header.Set("X-Http-Ja4h-Fingerprint", ja4hFP)

	if fp := fingerprint.GetTLSFingerprint(r); fp != nil {
		if ja3n := fp.JA3N(); ja3n != nil {
			r.Header.Set("X-Tls-Ja3n-Fingerprint", ja3n.String())
		}
		if ja4 := fp.JA4(); ja4 != nil {
			r.Header.Set("X-Tls-Ja4-Fingerprint", ja4.String())
		}
	}

	if tcpFP := fingerprint.GetTCPFingerprint(r); tcpFP != nil {
		r.Header.Set("X-Tcp-Ja4t-Fingerprint", tcpFP.String())
	}

	m := httpsnoop.CaptureMetrics(h, w, r)

	requestsPerDomain.WithLabelValues(host, r.Method, fmt.Sprint(m.Code)).Inc()
	responseTime.WithLabelValues(host).Observe(float64(m.Duration.Milliseconds()))

	slog.Info("request completed", "host", host, "method", r.Method, "response_code", m.Code, "duration_ms", m.Duration.Milliseconds())
}
