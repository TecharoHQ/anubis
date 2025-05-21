package thoth

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy"
	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/timeout"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

type Client struct {
	thothURL string

	conn    *grpc.ClientConn
	health  healthv1.HealthClient
	iptoasn iptoasnv1.IpToASNServiceClient
}

func New(ctx context.Context, thothURL, apiToken string) (*Client, error) {
	clMetrics := grpcprom.NewClientMetrics(
		grpcprom.WithClientHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120}),
		),
	)
	prometheus.DefaultRegisterer.Register(clMetrics)

	conn, err := grpc.DialContext(
		ctx,
		thothURL,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
		grpc.WithChainUnaryInterceptor(
			timeout.UnaryClientInterceptor(500*time.Millisecond),
			clMetrics.UnaryClientInterceptor(),
			authUnaryClientInterceptor(apiToken),
		),
		grpc.WithChainStreamInterceptor(
			clMetrics.StreamClientInterceptor(),
			authStreamClientInterceptor(apiToken),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("can't dial thoth at %s: %w", thothURL, err)
	}

	hc := healthv1.NewHealthClient(conn)

	resp, err := hc.Check(ctx, &healthv1.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("can't verify thoth health at %s: %w", thothURL, err)
	}

	if resp.Status != healthv1.HealthCheckResponse_SERVING {
		return nil, fmt.Errorf("thoth is not healthy, wanted %s but got %s", healthv1.HealthCheckResponse_SERVING, resp.Status)
	}

	return &Client{
		conn:    conn,
		health:  hc,
		iptoasn: NewIpToASNWithCache(iptoasnv1.NewIpToASNServiceClient(conn)),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) ASNCheckerFor(asns []uint32) policy.Checker {
	asnMap := map[uint32]struct{}{}
	var sb strings.Builder
	fmt.Fprintln(&sb, "ASNChecker")
	for _, asn := range asns {
		asnMap[asn] = struct{}{}
		fmt.Fprintln(&sb, "AS", asn)
	}

	return &ASNChecker{
		iptoasn: c.iptoasn,
		asns:    asnMap,
		hash:    internal.SHA256sum(sb.String()),
	}
}
