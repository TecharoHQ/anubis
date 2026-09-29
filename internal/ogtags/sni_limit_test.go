package ogtags

import (
	"fmt"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestSNIClientCacheBounded(t *testing.T) {
	cache := NewOGTagCache("https://example.com", config.OpenGraph{}, memory.New(t.Context()), TargetOptions{SNI: "auto"})
	first := cache.clientForSNI("stable.example.com")
	if cache.clientForSNI("stable.example.com") != first {
		t.Fatal("client not reused")
	}
	for n := 0; n < 1000; n++ {
		cache.clientForSNI(fmt.Sprintf("%d.example.com", n))
	}
	if len(cache.sniClients) > 128 {
		t.Fatalf("retained %d clients", len(cache.sniClients))
	}
}
