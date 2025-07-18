package entrypoint

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/TecharoHQ/anubis/cmd/osiris/internal/config"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"golang.org/x/sync/errgroup"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

type Options struct {
	ConfigFname string
}

func Main(ctx context.Context, opts Options) error {
	internal.SetHealth("osiris", healthv1.HealthCheckResponse_NOT_SERVING)

	var cfg config.Toplevel
	if err := hclsimple.DecodeFile(opts.ConfigFname, nil, &cfg); err != nil {
		return fmt.Errorf("can't read configuration file %s:\n\n%w", opts.ConfigFname, err)
	}

	if err := cfg.Valid(); err != nil {
		return fmt.Errorf("configuration file %s is invalid:\n\n%w", opts.ConfigFname, err)
	}

	rtr, err := NewRouter(cfg)
	if err != nil {
		return err
	}
	rtr.opts = opts
	go rtr.backgroundReloadConfig(ctx)

	g, gCtx := errgroup.WithContext(ctx)

	// HTTP
	g.Go(func() error {
		ln, err := net.Listen("tcp", cfg.Bind.HTTP)
		if err != nil {
			return fmt.Errorf("(HTTP) can't bind to tcp %s: %w", cfg.Bind.HTTP, err)
		}
		defer ln.Close()

		go func(ctx context.Context) {
			<-ctx.Done()
			ln.Close()
		}(ctx)

		slog.Info("listening", "for", "http", "bind", cfg.Bind.HTTP)

		return rtr.HandleHTTP(gCtx, ln)
	})

	// HTTPS
	g.Go(func() error {
		ln, err := net.Listen("tcp", cfg.Bind.HTTPS)
		if err != nil {
			return fmt.Errorf("(https) can't bind to tcp %s: %w", cfg.Bind.HTTPS, err)
		}
		defer ln.Close()

		go func(ctx context.Context) {
			<-ctx.Done()
			ln.Close()
		}(ctx)

		slog.Info("listening", "for", "https", "bind", cfg.Bind.HTTPS)

		return rtr.HandleHTTPS(gCtx, ln)
	})

	// Metrics
	g.Go(func() error {
		return rtr.ListenAndServeMetrics(gCtx, cfg.Bind.Metrics)
	})

	internal.SetHealth("osiris", healthv1.HealthCheckResponse_SERVING)

	return g.Wait()
}
