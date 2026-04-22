package metrics

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// writeKeypair generates a fresh self-signed cert + RSA key and writes them
// as PEM files in dir. Returns the paths and the cert's DER bytes so callers
// can identify which pair was loaded.
func writeKeypair(t *testing.T, dir, prefix string) (certPath, keyPath string, certDER []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "keypairreloader-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"keypairreloader-test"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}

	certPath = filepath.Join(dir, prefix+"cert.pem")
	keyPath = filepath.Join(dir, prefix+"key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return certPath, keyPath, der
}

func TestNewKeypairReloader(t *testing.T) {
	dir := t.TempDir()
	goodCert, goodKey, _ := writeKeypair(t, dir, "good-")

	garbagePath := filepath.Join(dir, "garbage.pem")
	if err := os.WriteFile(garbagePath, []byte("not a pem file"), 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	tests := []struct {
		name     string
		certPath string
		keyPath  string
		wantErr  error
		wantNil  bool
	}{
		{
			name:     "valid cert and key",
			certPath: goodCert,
			keyPath:  goodKey,
		},
		{
			name:     "missing cert file",
			certPath: filepath.Join(dir, "does-not-exist.pem"),
			keyPath:  goodKey,
			wantErr:  os.ErrNotExist,
			wantNil:  true,
		},
		{
			name:     "missing key file",
			certPath: goodCert,
			keyPath:  filepath.Join(dir, "does-not-exist-key.pem"),
			wantErr:  os.ErrNotExist,
			wantNil:  true,
		},
		{
			name:     "cert file is garbage",
			certPath: garbagePath,
			keyPath:  goodKey,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kpr, err := NewKeypairReloader(tt.certPath, tt.keyPath, discardLogger())

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
			if tt.wantErr == nil && !tt.wantNil && err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if tt.wantNil && kpr != nil {
				t.Errorf("kpr = %+v, want nil", kpr)
			}
			if !tt.wantNil && kpr == nil {
				t.Errorf("kpr is nil, want non-nil")
			}
		})
	}
}

func TestKeypairReloader_GetCertificate(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "returns loaded cert",
			run: func(t *testing.T) {
				dir := t.TempDir()
				certPath, keyPath, wantDER := writeKeypair(t, dir, "a-")

				kpr, err := NewKeypairReloader(certPath, keyPath, discardLogger())
				if err != nil {
					t.Fatalf("NewKeypairReloader: %v", err)
				}

				got, err := kpr.GetCertificate(nil)
				if err != nil {
					t.Fatalf("GetCertificate: %v", err)
				}
				if len(got.Certificate) == 0 || !bytes.Equal(got.Certificate[0], wantDER) {
					t.Errorf("GetCertificate returned wrong cert bytes")
				}
			},
		},
		{
			name: "reloads when mtime advances",
			run: func(t *testing.T) {
				dir := t.TempDir()
				certPath, keyPath, _ := writeKeypair(t, dir, "a-")

				kpr, err := NewKeypairReloader(certPath, keyPath, discardLogger())
				if err != nil {
					t.Fatalf("NewKeypairReloader: %v", err)
				}

				// Overwrite with a new pair at the same paths and bump mtime.
				newCertPath, newKeyPath, newDER := writeKeypair(t, dir, "b-")
				mustRename(t, newCertPath, certPath)
				mustRename(t, newKeyPath, keyPath)
				future := time.Now().Add(time.Hour)
				if err := os.Chtimes(certPath, future, future); err != nil {
					t.Fatalf("Chtimes: %v", err)
				}

				got, err := kpr.GetCertificate(nil)
				if err != nil {
					t.Fatalf("GetCertificate: %v", err)
				}
				if len(got.Certificate) == 0 || !bytes.Equal(got.Certificate[0], newDER) {
					t.Errorf("GetCertificate did not return reloaded cert")
				}
			},
		},
		{
			name: "does not reload when mtime unchanged",
			run: func(t *testing.T) {
				dir := t.TempDir()
				certPath, keyPath, originalDER := writeKeypair(t, dir, "a-")

				kpr, err := NewKeypairReloader(certPath, keyPath, discardLogger())
				if err != nil {
					t.Fatalf("NewKeypairReloader: %v", err)
				}

				// Overwrite the cert/key files with a *different* keypair, then
				// rewind mtime so the reloader must not pick up the change.
				newCertPath, newKeyPath, newDER := writeKeypair(t, dir, "b-")
				mustRename(t, newCertPath, certPath)
				mustRename(t, newKeyPath, keyPath)
				past := time.Unix(0, 0)
				if err := os.Chtimes(certPath, past, past); err != nil {
					t.Fatalf("Chtimes: %v", err)
				}

				got, err := kpr.GetCertificate(nil)
				if err != nil {
					t.Fatalf("GetCertificate: %v", err)
				}
				if len(got.Certificate) == 0 {
					t.Fatal("empty cert chain")
				}
				if bytes.Equal(got.Certificate[0], newDER) {
					t.Errorf("GetCertificate reloaded despite unchanged mtime")
				}
				if !bytes.Equal(got.Certificate[0], originalDER) {
					t.Errorf("GetCertificate did not return original cert")
				}
			},
		},
		{
			name: "does not panic when reload fails after mtime bump",
			run: func(t *testing.T) {
				dir := t.TempDir()
				certPath, keyPath, _ := writeKeypair(t, dir, "a-")

				kpr, err := NewKeypairReloader(certPath, keyPath, discardLogger())
				if err != nil {
					t.Fatalf("NewKeypairReloader: %v", err)
				}

				// Corrupt the cert file and bump mtime. maybeReload will fail.
				if err := os.WriteFile(certPath, []byte("not a pem file"), 0o600); err != nil {
					t.Fatalf("corrupt cert: %v", err)
				}
				future := time.Now().Add(time.Hour)
				if err := os.Chtimes(certPath, future, future); err != nil {
					t.Fatalf("Chtimes: %v", err)
				}

				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("GetCertificate panicked on reload failure: %v", r)
					}
				}()

				got, err := kpr.GetCertificate(nil)
				if err == nil {
					t.Errorf("GetCertificate returned nil err for corrupt cert; got %+v", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func mustRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("rename %q -> %q: %v", from, to, err)
	}
}
