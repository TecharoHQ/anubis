package internal

import (
	"context"

	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

var HealthSrv = health.NewServer()

func SetHealth(svc string, status healthv1.HealthCheckResponse_ServingStatus) {
	HealthSrv.SetServingStatus(svc, status)
}

func GetHealth(svc string) (healthv1.HealthCheckResponse_ServingStatus, bool) {
	st, err := HealthSrv.Check(context.Background(), &healthv1.HealthCheckRequest{
		Service: svc,
	})
	if err != nil {
		return healthv1.HealthCheckResponse_UNKNOWN, false
	}

	return st.GetStatus(), true
}
