package entrypoint

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/internal"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(healthz))

	internal.SetHealth("osiris", healthv1.HealthCheckResponse_NOT_SERVING)

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("wanted not ready but got %d", resp.StatusCode)
	}

	internal.SetHealth("osiris", healthv1.HealthCheckResponse_SERVING)

	resp, err = srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("wanted ready but got %d", resp.StatusCode)
	}
}

func TestReadyz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(readyz))

	internal.SetHealth("osiris", healthv1.HealthCheckResponse_NOT_SERVING)

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Errorf("wanted not ready but got %d", resp.StatusCode)
	}

	internal.SetHealth("osiris", healthv1.HealthCheckResponse_SERVING)

	resp, err = srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("wanted ready but got %d", resp.StatusCode)
	}
}
