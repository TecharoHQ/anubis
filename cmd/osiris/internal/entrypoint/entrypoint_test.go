package entrypoint

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMainGoodConfig(t *testing.T) {
	files, err := os.ReadDir("./testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range files {
		t.Run(st.Name(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cfg := loadConfig(t, filepath.Join("testdata", "good", st.Name()))

			go func(ctx context.Context) {
				if err := Main(ctx, Options{
					ConfigFname: filepath.Join("testdata", "good", st.Name()),
				}); err != nil {
					var netOpErr *net.OpError
					switch {
					case errors.Is(err, context.Canceled):
						// Context was canceled, this is expected
						return
					case errors.As(err, &netOpErr):
						// Network operation error occurred
						t.Logf("Network operation error: %v", netOpErr)
						return
					case errors.Is(err, http.ErrServerClosed):
						// Server was closed, this is expected
						return
					default:
						// Other unexpected error
						panic(err)
					}
				}
			}(ctx)

			wait := 5 * time.Millisecond

			for i := range make([]struct{}, 10) {
				if i != 0 {
					time.Sleep(wait)
					wait = wait * 2
				}

				t.Logf("try %d (wait=%s)", i+1, wait)

				resp, err := http.Get("http://localhost" + cfg.Bind.Metrics + "/readyz")
				if err != nil {
					continue
				}

				if resp.StatusCode != http.StatusOK {
					continue
				}

				cancel()
				return
			}

			t.Fatal("router initialization did not work")
		})
	}
}

func TestMainBadConfig(t *testing.T) {
	files, err := os.ReadDir("./testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range files {
		t.Run(st.Name(), func(t *testing.T) {
			if err := Main(t.Context(), Options{
				ConfigFname: filepath.Join("testdata", "bad", st.Name()),
			}); err == nil {
				t.Error("wanted an error but got none")
			} else {
				t.Log(err)
			}
		})
	}
}
