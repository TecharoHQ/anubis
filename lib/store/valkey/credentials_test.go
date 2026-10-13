package valkey

import (
	"encoding/json"
	"os"
	"strings"
	"io"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestResolveCredentials(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		password     string
		envUsername  *string
		envPassword  *string
		wantUsername string
		wantPassword string
	}{
		{
			name:         "preserve existing credentials",
			username:     "original-user",
			password:     "original-password",
			wantUsername: "original-user",
			wantPassword: "original-password",
		},
		{
			name:         "override credentials",
			username:     "original-user",
			password:     "original-password",
			envUsername:  strPtr("new-user"),
			envPassword:  strPtr("new-password"),
			wantUsername: "new-user",
			wantPassword: "new-password",
		},
		{
			name:         "password only",
			envPassword:  strPtr("secret"),
			wantPassword: "secret",
		},
		{
			name:        "empty environment variables override",
			username:    "original-user",
			password:    "original-password",
			envUsername: strPtr(""),
			envPassword: strPtr(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure test isolation, including when environment
			// variables have been exported by the developer.
			unsetEnvForTest(t, "ANUBIS_VALKEY_USERNAME")
			unsetEnvForTest(t, "ANUBIS_VALKEY_PASSWORD")

			if tt.envUsername != nil {
				t.Setenv("ANUBIS_VALKEY_USERNAME", *tt.envUsername)
			}
			if tt.envPassword != nil {
				t.Setenv("ANUBIS_VALKEY_PASSWORD", *tt.envPassword)
			}

			gotUsername, gotPassword := resolveCredentials(
				tt.username,
				tt.password,
			)

			if gotUsername != tt.wantUsername || gotPassword != tt.wantPassword {
				t.Errorf(
					"resolveCredentials() = (%q, %q), want (%q, %q)",
					gotUsername, gotPassword,
					tt.wantUsername, tt.wantPassword,
				)
			}
		})
	}
}

func TestFactoryAuthenticatedValkey(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
	}

	testcontainers.SkipIfProviderIsNotHealthy(t)

	const password = "test-valkey-password"

	valkeyC, err := testcontainers.Run(
		t.Context(),
		"valkey/valkey:8",
		// cspell:ignore requirepass
		testcontainers.WithCmd("--requirepass", password),
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
			wait.ForLog("Ready to accept connections"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, valkeyC)

	endpoint, err := valkeyC.PortEndpoint(t.Context(), "6379/tcp", "redis")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		url         string
		envPassword string
		wantError   bool
	}{
		{
			name:        "environment password authenticates",
			url:         endpoint,
			envPassword: password,
		},
		{
			name: "existing URL password still works",
			url:  strings.Replace(endpoint, "redis://", "redis://:"+password+"@", 1),
		},
		{
			name:        "incorrect environment password fails",
			url:         endpoint,
			envPassword: "wrong-password",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnvForTest(t, "ANUBIS_VALKEY_USERNAME")
			unsetEnvForTest(t, "ANUBIS_VALKEY_PASSWORD")

			if tt.name != "existing URL password still works" {
				t.Setenv("ANUBIS_VALKEY_PASSWORD", tt.envPassword)
			}

			config, err := json.Marshal(Config{URL: tt.url})
			if err != nil {
				t.Fatal(err)
			}

			_, err = (Factory{}).Build(t.Context(), config)

			if tt.wantError && err == nil {
				t.Fatal("expected authentication failure, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected successful connection, got %v", err)
			}
		})
	}
}
func TestFactoryNamedUserAuthentication(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires network egress")
	}

	testcontainers.SkipIfProviderIsNotHealthy(t)

	const (
		defaultPassword = "default-test-password"
		username        = "anubis"
		password        = "named-user-password"
	)

	valkeyC, err := testcontainers.Run(
		t.Context(),
		"valkey/valkey:8",
		// cspell:ignore requirepass
		testcontainers.WithCmd("--requirepass", defaultPassword),
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp"),
			wait.ForLog("Ready to accept connections"),
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, valkeyC)
	// cspell:ignore SETUSER
	exitCode, output, err := valkeyC.Exec(t.Context(), []string{
		"valkey-cli", "-a", defaultPassword,
		"ACL", "SETUSER", username,
		"on", ">" + password, "~*", "+@all",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := io.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 0 || !strings.Contains(string(result), "OK") {
		t.Fatalf("failed to create ACL user: exit=%d, output=%s", exitCode, result)
	}

	endpoint, err := valkeyC.PortEndpoint(t.Context(), "6379/tcp", "redis")
	if err != nil {
		t.Fatal(err)
	}

	config, err := json.Marshal(Config{URL: endpoint})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		username  string
		password  string
		wantError bool
	}{
		{
			name:     "named user authenticates",
			username: username,
			password: password,
		},
		{
			name:      "incorrect username fails",
			username:  "unknown-user",
			password:  password,
			wantError: true,
		},
		{
			name:      "incorrect password fails",
			username:  username,
			password:  "incorrect-password",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unsetEnvForTest(t, "ANUBIS_VALKEY_USERNAME")
			unsetEnvForTest(t, "ANUBIS_VALKEY_PASSWORD")

			t.Setenv("ANUBIS_VALKEY_USERNAME", tt.username)
			t.Setenv("ANUBIS_VALKEY_PASSWORD", tt.password)

			_, err := (Factory{}).Build(t.Context(), config)

			if tt.wantError && err == nil {
				t.Fatal("expected authentication failure, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected successful connection, got %v", err)
			}
		})
	}
}
func strPtr(value string) *string {
	return &value
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	oldValue, existed := os.LookupEnv(key)

	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		var err error
		if existed {
			err = os.Setenv(key, oldValue)
		} else {
			err = os.Unsetenv(key)
		}

		if err != nil {
			t.Errorf("failed to restore environment variable %s: %v", key, err)
		}
	})
}
