package metrics

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	Config *config.Metrics
	Log    *slog.Logger
}

func (s *Server) Run(ctx context.Context, done func()) {
	defer done()
	lg := s.Log.With("subsystem", "metrics")

	if err := s.run(ctx, lg); err != nil {
		lg.Error("can't serve metrics server", "err", err)
	}
}

func (s *Server) run(ctx context.Context, lg *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		st, ok := internal.GetHealth("anubis")
		if !ok {
			slog.Error("health service anubis does not exist, file a bug")
		}

		switch st {
		case healthv1.HealthCheckResponse_NOT_SERVING:
			http.Error(w, "NOT OK", http.StatusInternalServerError)
			return
		case healthv1.HealthCheckResponse_SERVING:
			fmt.Fprintln(w, "OK")
			return
		default:
			http.Error(w, "UNKNOWN", http.StatusFailedDependency)
			return
		}
	})

	srv := http.Server{
		Handler:  mux,
		ErrorLog: internal.GetFilteredHTTPLogger(),
	}

	ln, metricsURL, err := internal.SetupListener(s.Config.Bind, s.Config.Network, s.Config.SocketMode)
	if err != nil {
		return fmt.Errorf("can't setup listener: %w", err)
	}

	defer ln.Close()

	if s.Config.TLS != nil {
		kpr, err := NewKeypairReloader(s.Config.TLS.Certificate, s.Config.TLS.Key, lg)
		if err != nil {
			return fmt.Errorf("can't setup keypair reloader: %w", err)
		}

		srv.TLSConfig = &tls.Config{
			GetCertificate: kpr.GetCertificate,
		}

		if s.Config.TLS.CA != "" {
			caCert, err := os.ReadFile(s.Config.TLS.CA)
			if err != nil {
				return fmt.Errorf("%w %s: %w", config.ErrCantReadFile, s.Config.TLS.CA, err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("%w %s", config.ErrInvalidMetricsCACertificate, s.Config.TLS.CA)
			}

			srv.TLSConfig.ClientCAs = certPool
			srv.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	lg.Debug("listening for metrics", "url", metricsURL)

	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(c); err != nil {
			lg.Error("can't shut down metrics server", "err", err)
		}
	}()

	switch s.Config.TLS != nil {
	case true:
		if err := srv.ServeTLS(ln, "", ""); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("can't serve TLS metrics server: %w", err)
		}
	case false:
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("can't serve metrics server: %w", err)
		}
	}

	return nil
}
