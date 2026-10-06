package test

import (
	"net/http"
	"testing"
)

func TestIntegrationServerLoadsDefaultPolicy(t *testing.T) {
	server := spawnAnubisWithPolicy(t, "", "", "")
	request, err := http.NewRequest(http.MethodGet, server+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Real-IP", "127.0.0.1")
	request.Header.Set("User-Agent", "Sephiroth")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}
