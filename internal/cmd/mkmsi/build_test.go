package main

import (
	"errors"
	"testing"
)

func TestVersionFromZipName(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{
			name: "dev build with path",
			in:   "var/win/anubis-1.26.2-1-gec3cce8a-dev-windows-amd64.zip",
			want: "1.26.2-1-gec3cce8a-dev",
		},
		{
			name: "tagged release",
			in:   "anubis-1.26.2-windows-amd64.zip",
			want: "1.26.2",
		},
		{
			name: "arm64",
			in:   "anubis-1.26.2-windows-arm64.zip",
			want: "1.26.2",
		},
		{name: "not an anubis zip", in: "something-else.zip", wantErr: ErrBadVersion},
		{name: "no platform suffix", in: "anubis-1.26.2.zip", wantErr: ErrBadVersion},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := versionFromZipName(tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("versionFromZipName(%q): got error %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("versionFromZipName(%q): unexpected error: %v", tt.in, err)
			}

			if got != tt.want {
				t.Errorf("versionFromZipName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCheckWixlOutput(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "clean", in: ""},
		{name: "harmless chatter", in: "building anubis.msi\n"},
		{
			name:    "dropped attribute",
			in:      "(wixl:1234): GLib-GObject-CRITICAL **: object class 'WixlWixFile' has no property named 'NeverOverwrite'\n",
			wantErr: ErrWixlDroppedAttribute,
		},
		{
			name:    "critical among other lines",
			in:      "fine\n** (wixl:1): CRITICAL **: assertion failed\nalso fine\n",
			wantErr: ErrWixlDroppedAttribute,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// checkWixlOutput is a pure string-matching function over
			// captured sample wixl stderr; it never shells out to wixl
			// itself, so unlike msi_test.go's tests this one needs no
			// requireTool gate and must always run.
			err := checkWixlOutput(tt.in)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("checkWixlOutput(%q): unexpected error: %v", tt.in, err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("checkWixlOutput(%q): got %v, want %v", tt.in, err, tt.wantErr)
			}
		})
	}
}
