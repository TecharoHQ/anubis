package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAutomaticSNI(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); seen[r.Host] = r.TLS.ServerName; mu.Unlock() }))
	target.TLS = &tls.Config{}
	target.StartTLS()
	defer target.Close()
	proxy, err := makeReverseProxy(target.URL, "auto", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, host := range []string{"first.example", "second.example", "third.example:443"} {
		wg.Go(func() {
			r := httptest.NewRequest("GET", "https://"+host+"/", nil)
			proxy.ServeHTTP(httptest.NewRecorder(), r)
		})
	}
	wg.Wait()
	for host, sni := range seen {
		if sni != strings.Split(host, ":")[0] {
			t.Errorf("host %s used SNI %s", host, sni)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("got %d requests", len(seen))
	}
}
