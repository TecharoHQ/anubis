package valkey

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/store/storetest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func init() {
	internal.UnbreakDocker()
}

func TestImpl(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
		return
	}

	testcontainers.SkipIfProviderIsNotHealthy(t)

	req := testcontainers.ContainerRequest{
		Image:      "valkey/valkey:8",
		WaitingFor: wait.ForLog("Ready to accept connections"),
	}
	valkeyC, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	testcontainers.CleanupContainer(t, valkeyC)
	if err != nil {
		t.Fatal(err)
	}

	containerIP, err := valkeyC.ContainerIP(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(Config{
		URL: fmt.Sprintf("redis://%s:6379/0", containerIP),
	})
	if err != nil {
		t.Fatal(err)
	}

	storetest.Common(t, Factory{}, json.RawMessage(data))
}
