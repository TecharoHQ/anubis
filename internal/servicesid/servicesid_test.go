package servicesid

import "testing"

func TestServiceSID(t *testing.T) {
	for _, tt := range []struct {
		name    string
		service string
		want    string
	}{
		{
			// TrustedInstaller ships with Windows and its SID is published in
			// enough places to serve as a known-answer test for the whole
			// derivation. If this case fails, the arithmetic is wrong.
			name:    "TrustedInstaller matches the published SID",
			service: "TrustedInstaller",
			want:    "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464",
		},
		{
			// Pins the hardcoded constant to the derivation. Renaming the
			// service without recomputing the SID fails here rather than on a
			// Windows box, where it would surface as the service being unable
			// to read its own configuration.
			name:    "the pinned Anubis SID is the derived one",
			service: AnubisServiceName,
			want:    AnubisServiceSID,
		},
		{
			// The name is uppercased before hashing, so case in the service
			// name must not reach the digest.
			name:    "case in the service name does not matter",
			service: "aNuBiS",
			want:    AnubisServiceSID,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Encode(tt.service); got != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.service, got, tt.want)
			}
		})
	}
}
