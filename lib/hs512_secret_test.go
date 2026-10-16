package lib

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestHS512SecretMinimumLength(t *testing.T) {
	for _, size := range []int{0, 1, 32, 63, 64, 128} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			secret := bytes.Repeat([]byte("x"), size)
			if size == 0 {
				secret = nil
			}
			srv, err := New(Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0), HS512Secret: secret})
			if size > 0 && size < 64 {
				if err == nil {
					t.Errorf("accepted %d-byte HS512 key", size)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := srv.signJWT(jwt.MapClaims{}); err != nil {
				t.Fatalf("valid signing configuration failed: %v", err)
			}
		})
	}
}
