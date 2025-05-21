package thoth

import (
	"context"
	"crypto/tls"
	"fmt"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
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
		iptoasn: iptoasnv1.NewIpToASNServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
