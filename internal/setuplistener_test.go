package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseBindNetFromAddr(t *testing.T) {
	for _, tt := range []struct {
		name    string
		address string
		wantErr bool
		network string
		bind    string
	}{
		{
			name:    "simple tcp",
			address: "localhost:9090",
			wantErr: false,
			network: "tcp",
			bind:    "localhost:9090",
		},
		{
			name:    "simple unix",
			address: "unix:///tmp/foo.sock",
			wantErr: false,
			network: "unix",
			bind:    "/tmp/foo.sock",
		},
		{
			name:    "invalid network",
			address: "foo:///tmp/bar.sock",
			wantErr: true,
		},
		{
			name:    "tcp uri",
			address: "tcp://[::]:9090",
			wantErr: false,
			network: "tcp",
			bind:    "[::]:9090",
		},
		{
			name:    "http uri",
			address: "http://[::]:9090",
			wantErr: false,
			network: "tcp",
			bind:    "[::]:9090",
		},
		{
			name:    "https uri",
			address: "https://[::]:9090",
			wantErr: false,
			network: "tcp",
			bind:    "[::]:9090",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			network, bind, err := parseBindNetFromAddr(tt.address)

			switch {
			case tt.wantErr && err == nil:
				t.Errorf("parseBindNetFromAddr(%q) should have errored but did not", tt.address)
			case !tt.wantErr && err != nil:
				t.Errorf("parseBindNetFromAddr(%q) threw an error: %v", tt.address, err)
			}

			if network != tt.network {
				t.Errorf("parseBindNetFromAddr(%q) wanted network: %q, got: %q", tt.address, tt.network, network)
			}

			if bind != tt.bind {
				t.Errorf("parseBindNetFromAddr(%q) wanted bind: %q, got: %q", tt.address, tt.bind, bind)
			}
		})
	}
}

func TestSetupListener(t *testing.T) {
	td := t.TempDir()

	for _, tt := range []struct {
		name                         string
		network, address, socketMode string
		wantErr                      bool
		socketURLPrefix              string
	}{
		{
			name:            "simple tcp",
			network:         "",
			address:         ":0",
			wantErr:         false,
			socketURLPrefix: "http://localhost:",
		},
		{
			name:            "simple unix",
			network:         "",
			address:         "unix://" + filepath.Join(td, "a"),
			socketMode:      "0770",
			wantErr:         false,
			socketURLPrefix: "unix:" + filepath.Join(td, "a"),
		},
		{
			name:            "tcp",
			network:         "tcp",
			address:         ":0",
			wantErr:         false,
			socketURLPrefix: "http://localhost:",
		},
		{
			name:            "udp",
			network:         "udp",
			address:         ":0",
			wantErr:         true,
			socketURLPrefix: "http://localhost:",
		},
		{
			name:            "unix socket",
			network:         "unix",
			socketMode:      "0770",
			address:         filepath.Join(td, "a"),
			wantErr:         false,
			socketURLPrefix: "unix:" + filepath.Join(td, "a"),
		},
		{
			name:            "invalid socket mode",
			network:         "unix",
			socketMode:      "taco bell",
			address:         filepath.Join(td, "a"),
			wantErr:         true,
			socketURLPrefix: "unix:" + filepath.Join(td, "a"),
		},
		{
			name:            "empty socket mode",
			network:         "unix",
			socketMode:      "",
			address:         filepath.Join(td, "a"),
			wantErr:         true,
			socketURLPrefix: "unix:" + filepath.Join(td, "a"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ln, socketURL, err := SetupListener(tt.network, tt.address, tt.socketMode)
			switch {
			case tt.wantErr && err == nil:
				t.Errorf("SetupListener(%q, %q, %q) should have errored but did not", tt.network, tt.address, tt.socketMode)
			case !tt.wantErr && err != nil:
				t.Fatalf("SetupListener(%q, %q, %q) threw an error: %v", tt.network, tt.address, tt.socketMode, err)
			}

			if ln != nil {
				defer ln.Close()
			}

			if !tt.wantErr && !strings.HasPrefix(socketURL, tt.socketURLPrefix) {
				t.Errorf("SetupListener(%q, %q, %q) should have returned a URL with prefix %q but got: %q", tt.network, tt.address, tt.socketMode, tt.socketURLPrefix, socketURL)
			}

			if tt.socketMode != "" {
				mode, err := strconv.ParseUint(tt.socketMode, 8, 0)
				if err != nil {
					return
				}

				sockPath := strings.TrimPrefix(socketURL, "unix:")
				st, err := os.Stat(sockPath)
				if err != nil {
					t.Fatalf("can't os.Stat(%q): %v", sockPath, err)
				}

				if st.Mode().Perm() != fs.FileMode(mode) {
					t.Errorf("file mode of %q should be %s but is actually %s", sockPath, strconv.FormatUint(mode, 8), strconv.FormatUint(uint64(st.Mode()), 8))
				}
			}
		})
	}
}
