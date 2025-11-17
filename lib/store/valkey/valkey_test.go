package valkey

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/TecharoHQ/anubis/lib/store/storetest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestImpl(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	testcontainers.SkipIfProviderIsNotHealthy(t)

	valkeyC, err := testcontainers.Run(
		t.Context(), "valkey/valkey:8",
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
			wait.ForLog("Ready to accept connections"),
		),
	)
	testcontainers.CleanupContainer(t, valkeyC)
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := valkeyC.PortEndpoint(t.Context(), "6379/tcp", "redis")
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(Config{
		URL: endpoint,
	})
	if err != nil {
		t.Fatal(err)
	}

	storetest.Common(t, Factory{}, json.RawMessage(data))
}
