package metrics

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type KeypairReloader struct {
	certMu   sync.RWMutex
	cert     *tls.Certificate
	certPath string
	keyPath  string
	modTime  time.Time
	lg       *slog.Logger
}

func NewKeypairReloader(certPath, keyPath string, lg *slog.Logger) (*KeypairReloader, error) {
	result := &KeypairReloader{
		certPath: certPath,
		keyPath:  keyPath,
		lg:       lg,
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	result.cert = &cert

	st, err := os.Stat(certPath)
	if err != nil {
		return nil, err
	}
	result.modTime = st.ModTime()

	return result, nil
}

func (kpr *KeypairReloader) maybeReload() error {
	kpr.lg.Debug("loading new keypair", "cert", kpr.certPath, "key", kpr.keyPath)
	newCert, err := tls.LoadX509KeyPair(kpr.certPath, kpr.keyPath)
	if err != nil {
		return err
	}

	st, err := os.Stat(kpr.certPath)
	if err != nil {
		return err
	}

	kpr.certMu.Lock()
	defer kpr.certMu.Unlock()
	kpr.cert = &newCert
	kpr.modTime = st.ModTime()

	return nil
}

func (kpr *KeypairReloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	st, err := os.Stat(kpr.certPath)
	if err != nil {
		return nil, fmt.Errorf("stat(%q): %w", kpr.certPath, err)
	}

	kpr.certMu.RLock()
	needsReload := st.ModTime().After(kpr.modTime)
	kpr.certMu.RUnlock()

	if needsReload {
		if err := kpr.maybeReload(); err != nil {
			return nil, fmt.Errorf("reload cert: %w", err)
		}
	}

	kpr.certMu.RLock()
	defer kpr.certMu.RUnlock()
	return kpr.cert, nil
}
