package main

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
)

func serveHealthz(t *testing.T, network, address string, status int, auth *config.MetricsBasicAuth) string {
	t.Helper()

	ln, err := net.Listen(network, address)
	if err != nil {
		t.Fatalf("net.Listen(%q, %q): %v", network, address, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			u, p, ok := r.BasicAuth()
			if !ok || u != auth.Username || p != auth.Password {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		w.WriteHeader(status)
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	t.Cleanup(func() { _ = srv.Close() })

	return ln.Addr().String()
}

// shortTempDir keeps unix socket paths under the 104 byte limit on macOS.
func shortTempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "anubis")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return dir
}

func TestCheckHealth(t *testing.T) {
	auth := &config.MetricsBasicAuth{Username: "user", Password: "hunter2"}

	for _, tt := range []struct {
		name    string
		network string
		status  int
		auth    *config.MetricsBasicAuth
		bind    func(t *testing.T, addr string) string
		wantErr bool
	}{
		{
			name:    "tcp port only",
			network: "tcp",
			status:  http.StatusOK,
			bind: func(t *testing.T, addr string) string {
				_, port, _ := net.SplitHostPort(addr)
				return ":" + port
			},
		},
		{
			name:    "tcp host and port",
			network: "tcp",
			status:  http.StatusOK,
			bind:    func(t *testing.T, addr string) string { return addr },
		},
		{
			name:    "unix socket",
			network: "unix",
			status:  http.StatusOK,
			bind:    func(t *testing.T, addr string) string { return addr },
		},
		{
			name:    "basic auth",
			network: "tcp",
			status:  http.StatusOK,
			auth:    auth,
			bind:    func(t *testing.T, addr string) string { return addr },
		},
		{
			name:    "not serving",
			network: "tcp",
			status:  http.StatusInternalServerError,
			bind:    func(t *testing.T, addr string) string { return addr },
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			listenAddr := "127.0.0.1:0"
			if tt.network == "unix" {
				listenAddr = filepath.Join(shortTempDir(t), "m.sock")
			}

			addr := serveHealthz(t, tt.network, listenAddr, tt.status, tt.auth)

			err := checkHealth(&config.Metrics{
				Bind:      tt.bind(t, addr),
				Network:   tt.network,
				BasicAuth: tt.auth,
			})

			if tt.wantErr && err == nil {
				t.Fatal("checkHealth should have errored but did not")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("checkHealth: %v", err)
			}
		})
	}
}

func TestHealthCheckMetricsConfigPrefersPolicy(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "m.sock")
	fname := filepath.Join(t.TempDir(), "policy.yaml")

	policy := `bots:
  - name: everything
    path_regex: .*
    action: ALLOW
metrics:
  bind: "` + sock + `"
  network: unix
  socketMode: "0660"
`
	if err := os.WriteFile(fname, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}

	oldFname := *policyFname
	*policyFname = fname
	t.Cleanup(func() { *policyFname = oldFname })

	cfg, err := healthCheckMetricsConfig()
	if err != nil {
		t.Fatalf("healthCheckMetricsConfig: %v", err)
	}

	if cfg.Network != "unix" || cfg.Bind != sock {
		t.Fatalf("wanted unix %q from the policy file, got %q %q", sock, cfg.Network, cfg.Bind)
	}

	serveHealthz(t, "unix", sock, http.StatusOK, nil)
	if err := checkHealth(cfg); err != nil {
		t.Fatalf("checkHealth: %v", err)
	}
}
