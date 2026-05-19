package metrics

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
)

func TestMetricsPprofCmdlineExposedWithoutAuthentication(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	srv := &Server{
		Config: &config.Metrics{Network: "tcp", Bind: addr},
		Log:    slog.Default(),
	}
	go srv.Run(ctx, func() { close(done) })

	url := "http://" + addr + "/debug/pprof/cmdline"
	var body []byte
	resp, err := http.Get(url)
	if err == nil {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("can't read body: %v", err)
		}
		defer resp.Body.Close()
	}
	time.Sleep(50 * time.Millisecond)
	if strings.Contains(string(body), "metrics.test") {
		t.Fatalf("pprof is enabled by default, cmdline process arguments: %q", string(body))
	}
	cancel()
	<-done
}
