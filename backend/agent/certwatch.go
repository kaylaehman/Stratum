package agent

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// certWatcher periodically checks whether the TLS cert files have changed on
// disk and rebuilds the *tls.Config when they do.  The new config is applied
// to the Manager so the next outbound connection uses the rotated credentials;
// existing connections are torn down by invalidating the cached client.
type certWatcher struct {
	certFile string
	keyFile  string
	caFile   string

	mu      sync.RWMutex
	tlsCfg  *tls.Config
	hashes  certHashes // fingerprints of the three files at last load
	mgr     *Manager
	logger  *slog.Logger
	pollInt time.Duration // how often to check; production default = 60s
}

type certHashes struct{ cert, key, ca [32]byte }

// newCertWatcher creates a certWatcher and performs an initial load of the TLS
// config from the given file paths.  Returns an error if the initial load fails.
func newCertWatcher(certFile, keyFile, caFile string, mgr *Manager, logger *slog.Logger) (*certWatcher, error) {
	cw := &certWatcher{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
		mgr:      mgr,
		logger:   logger,
		pollInt:  60 * time.Second,
	}
	if err := cw.reload(); err != nil {
		return nil, err
	}
	return cw, nil
}

// TLSConfig returns the most recently loaded *tls.Config.
func (cw *certWatcher) TLSConfig() *tls.Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.tlsCfg
}

// Run polls for cert-file changes until ctx is cancelled.
// Intended to run as a background goroutine.
func (cw *certWatcher) Run(ctx interface{ Done() <-chan struct{} }) {
	ticker := time.NewTicker(cw.pollInt)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cw.checkAndReload()
		}
	}
}

// checkAndReload reloads only when at least one file hash differs.
func (cw *certWatcher) checkAndReload() {
	newH, err := hashFiles(cw.certFile, cw.keyFile, cw.caFile)
	if err != nil {
		cw.logger.Warn("certwatch: hash cert files", "error", err)
		return
	}

	cw.mu.RLock()
	same := newH == cw.hashes
	cw.mu.RUnlock()
	if same {
		return
	}

	if err := cw.reload(); err != nil {
		cw.logger.Warn("certwatch: reload TLS config", "error", err)
		return
	}

	// Tear down all cached agent connections so they re-dial with new certs.
	cw.mgr.InvalidateAll()
	cw.logger.Info("certwatch: TLS certificates rotated; all agent connections invalidated")
}

// reload builds a fresh *tls.Config from disk and stores it together with the
// new file hashes.
func (cw *certWatcher) reload() error {
	cfg, err := ClientTLSConfig(cw.certFile, cw.keyFile, cw.caFile)
	if err != nil {
		return fmt.Errorf("certwatch: build TLS config: %w", err)
	}
	h, err := hashFiles(cw.certFile, cw.keyFile, cw.caFile)
	if err != nil {
		return fmt.Errorf("certwatch: hash files: %w", err)
	}

	cw.mu.Lock()
	cw.tlsCfg = cfg
	cw.hashes = h
	cw.mu.Unlock()
	return nil
}

// hashFiles returns sha256 fingerprints of up to three files.  An empty path
// yields a zero hash rather than an error, so optional files are handled
// gracefully.
func hashFiles(files ...string) (certHashes, error) {
	var h certHashes
	hashes := []*[32]byte{&h.cert, &h.key, &h.ca}
	for i, f := range files {
		if f == "" {
			continue
		}
		sum, err := fileHash(f)
		if err != nil {
			return certHashes{}, err
		}
		*hashes[i] = sum
	}
	return h, nil
}

func fileHash(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [32]byte{}, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
