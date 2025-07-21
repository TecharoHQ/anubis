package entrypoint

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/cmd/osiris/internal/config"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

func loadConfig(t *testing.T, fname string) config.Toplevel {
	t.Helper()

	var cfg config.Toplevel
	if err := hclsimple.DecodeFile(fname, nil, &cfg); err != nil {
		t.Fatalf("can't read configuration file %s: %v", fname, err)
	}

	if err := cfg.Valid(); err != nil {
		t.Errorf("configuration file %s is invalid: %v", "./testdata/selfsigned.hcl", err)
	}

	return cfg
}

func newRouter(t *testing.T, cfg config.Toplevel) *Router {
	t.Helper()

	rtr, err := NewRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return rtr
}

func TestNewRouter(t *testing.T) {
	cfg := loadConfig(t, "./testdata/good/selfsigned.hcl")
	rtr := newRouter(t, cfg)

	srv := httptest.NewServer(rtr)
	defer srv.Close()
}

func TestNewRouterFails(t *testing.T) {
	cfg := loadConfig(t, "./testdata/good/selfsigned.hcl")

	cfg.Domains = append(cfg.Domains, config.Domain{
		Name: "test1.internal",
		TLS: config.TLS{
			Cert: "./testdata/tls/invalid.crt",
			Key:  "./testdata/tls/invalid.key",
		},
		Target:       cfg.Domains[0].Target,
		HealthTarget: cfg.Domains[0].HealthTarget,
	})

	rtr, err := NewRouter(cfg)
	if err == nil {
		t.Fatal("wanted an error but got none")
	}

	srv := httptest.NewServer(rtr)
	defer srv.Close()
}

func TestRouterSetConfig(t *testing.T) {
	for _, tt := range []struct {
		name        string
		configFname string
		mutation    func(cfg config.Toplevel) config.Toplevel
		err         error
	}{
		{
			name:        "basic",
			configFname: "./testdata/good/selfsigned.hcl",
			mutation: func(cfg config.Toplevel) config.Toplevel {
				return cfg
			},
		},
		{
			name:        "all schemes",
			configFname: "./testdata/good/selfsigned.hcl",
			mutation: func(cfg config.Toplevel) config.Toplevel {
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "http.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "http://[::1]:3000",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "https.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "https://[::1]:3000",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "h2c.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "h2c://[::1]:3000",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "unix.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "unix://foo.sock",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})

				return cfg
			},
		},
		{
			name:        "invalid TLS",
			configFname: "./testdata/good/selfsigned.hcl",
			mutation: func(cfg config.Toplevel) config.Toplevel {
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name: "test1.internal",
					TLS: config.TLS{
						Cert: "./testdata/tls/invalid.crt",
						Key:  "./testdata/tls/invalid.key",
					},
					Target:       cfg.Domains[0].Target,
					HealthTarget: cfg.Domains[0].HealthTarget,
				})

				return cfg
			},
			err: ErrInvalidTLSKeypair,
		},
		{
			name:        "target is not a valid URL",
			configFname: "./testdata/good/selfsigned.hcl",
			mutation: func(cfg config.Toplevel) config.Toplevel {
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "test1.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "http://[::1:443",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})

				return cfg
			},
			err: ErrTargetInvalid,
		},
		{
			name:        "invalid target scheme",
			configFname: "./testdata/good/selfsigned.hcl",
			mutation: func(cfg config.Toplevel) config.Toplevel {
				cfg.Domains = append(cfg.Domains, config.Domain{
					Name:         "test1.internal",
					TLS:          cfg.Domains[0].TLS,
					Target:       "foo://",
					HealthTarget: cfg.Domains[0].HealthTarget,
				})

				return cfg
			},
			err: ErrNoHandler,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadConfig(t, tt.configFname)
			rtr := newRouter(t, cfg)

			cfg = tt.mutation(cfg)

			if err := rtr.setConfig(cfg); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong error from rtr.setConfig function")
			}
		})
	}
}

type ackHandler struct {
	ack bool
}

func (ah *ackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ah.ack = true
	fmt.Fprintln(w, "OK")
}

func (ah *ackHandler) Reset() {
	ah.ack = false
}

func newUnixServer(t *testing.T, h http.Handler) string {
	sockName := filepath.Join(t.TempDir(), "s")
	ln, err := net.Listen("unix", sockName)
	if err != nil {
		t.Fatalf("can't listen on %s: %v", sockName, err)
	}
	t.Cleanup(func() {
		ln.Close()
		os.Remove(sockName)
	})

	go func(ctx context.Context) {
		srv := &http.Server{
			Handler: h,
		}

		go func() {
			<-ctx.Done()
			srv.Close()
		}()

		srv.Serve(ln)
	}(t.Context())

	return "unix://" + sockName
}

func TestRouterGetCertificate(t *testing.T) {
	cfg := loadConfig(t, "./testdata/good/selfsigned.hcl")
	rtr := newRouter(t, cfg)

	for _, tt := range []struct {
		domainName string
		err        error
	}{
		{
			domainName: "osiris.local.cetacean.club",
		},
		{
			domainName: "whacky-fun.local",
			err:        ErrNoCert,
		},
	} {
		t.Run(tt.domainName, func(t *testing.T) {
			if _, err := rtr.GetCertificate(&tls.ClientHelloInfo{ServerName: tt.domainName}); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong error from rtr.GetCertificate")
			}
		})
	}
}

func TestRouterServeAllProtocols(t *testing.T) {
	cfg := loadConfig(t, "./testdata/good/all_protocols.hcl")

	httpAckHandler := &ackHandler{}
	httpsAckHandler := &ackHandler{}
	h2cAckHandler := &ackHandler{}
	unixAckHandler := &ackHandler{}

	httpSrv := httptest.NewServer(httpAckHandler)
	httpsSrv := httptest.NewTLSServer(httpsAckHandler)
	h2cSrv := newH2cServer(t, h2cAckHandler)
	unixPath := newUnixServer(t, unixAckHandler)

	cfg.Domains[0].Target = httpSrv.URL
	cfg.Domains[1].Target = httpsSrv.URL
	cfg.Domains[2].Target = strings.ReplaceAll(h2cSrv.URL, "http:", "h2c:")
	cfg.Domains[3].Target = unixPath

	// enc := json.NewEncoder(os.Stderr)
	// enc.SetIndent("", "  ")
	// enc.Encode(cfg)

	rtr := newRouter(t, cfg)

	cli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	t.Run("plain http", func(t *testing.T) {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ln.Close()
		})

		go rtr.HandleHTTP(t.Context(), ln)

		serverURL := "http://" + ln.Addr().String()
		t.Log(serverURL)

		for _, d := range cfg.Domains {
			t.Run(d.Name, func(t *testing.T) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, serverURL, nil)
				if err != nil {
					t.Fatal(err)
				}

				req.Host = d.Name

				resp, err := cli.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					t.Fatalf("wrong status code %d", resp.StatusCode)
				}
			})
		}
	})
}
