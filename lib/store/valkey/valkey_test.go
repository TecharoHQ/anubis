package valkey

import (
	"encoding/json"
	"errors"
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

func TestFactoryValid(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError error
	}{
		{
			name:        "empty config",
			jsonData:    `{}`,
			expectError: ErrNoURL,
		},
		{
			name:        "valid URL only",
			jsonData:    `{"url": "redis://localhost:6379"}`,
			expectError: nil,
		},
		{
			name:        "invalid URL",
			jsonData:    `{"url": "invalid-url"}`,
			expectError: ErrBadURL,
		},
		{
			name:        "valid sentinel config",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": ["localhost:26379"], "password": "mypass"}}`,
			expectError: nil,
		},
		{
			name:        "sentinel missing masterName",
			jsonData:    `{"sentinel": {"addr": ["localhost:26379"], "password": "mypass"}}`,
			expectError: ErrSentinelMasterNameRequired,
		},
		{
			name:        "sentinel missing addr",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "password": "mypass"}}`,
			expectError: ErrSentinelAddrRequired,
		},
		{
			name:        "sentinel empty addr",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": [""], "password": "mypass"}}`,
			expectError: ErrSentinelAddrEmpty,
		},
		{
			name:        "sentinel missing password",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": ["localhost:26379"]}}`,
			expectError: nil,
		},
		{
			name:        "sentinel with optional fields",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": ["localhost:26379"], "password": "mypass", "clientName": "myclient", "username": "myuser"}}`,
			expectError: nil,
		},
		{
			name:        "sentinel single address (not array)",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": "localhost:26379", "password": "mypass"}}`,
			expectError: nil,
		},
		{
			name:        "sentinel mixed empty and valid addresses",
			jsonData:    `{"sentinel": {"masterName": "mymaster", "addr": ["", "localhost:26379", ""], "password": "mypass"}}`,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := Factory{}
			err := factory.Valid(json.RawMessage(tt.jsonData))

			if tt.expectError == nil {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got: %v", tt.expectError, err)
				}
			}
		})
	}
}
