// Package certwatcher watches a TLS certificate and key pair on disk and
// reloads them on change, so a running server can pick up renewed certificates
// without a restart. It is a lightweight, dependency-free replacement for
// sigs.k8s.io/controller-runtime/pkg/certwatcher, keeping the same behaviour
// (initial load, fsnotify-driven reload, re-watching after atomic swaps, and a
// periodic poll as a safety net) while relying only on fsnotify.
package certwatcher

import (
	"bytes"
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/scality/go-errors"
	"github.com/scality/static-oci-registry/pkg/domain"
)

// defaultWatchInterval is how often the certificate is re-read from disk as a
// safety net, independent of filesystem events.
const defaultWatchInterval = 120 * time.Second

// watchAddTimeout bounds how long Start waits for the certificate and key files
// to become watchable (they may not exist yet at startup).
const (
	watchAddTimeout  = 10 * time.Second
	watchAddInterval = 1 * time.Second
)

// CertWatcher watches certificate and key files for changes. GetCertificate
// always returns the cached certificate; Start periodically re-reads the files
// and reacts to filesystem events to refresh that cache.
type CertWatcher struct {
	mu sync.RWMutex

	logger *slog.Logger

	currentCert *tls.Certificate
	watcher     *fsnotify.Watcher
	interval    time.Duration

	certPath string
	keyPath  string

	cachedKeyPEMBlock []byte
}

// New returns a new CertWatcher watching the given certificate and key files.
// The files are read once immediately so a certificate is available before the
// watcher is started.
func New(logger *slog.Logger, certPath, keyPath string) (*CertWatcher, error) {
	cw := &CertWatcher{
		logger:   logger,
		certPath: certPath,
		keyPath:  keyPath,
		interval: defaultWatchInterval,
	}

	// Initial read of certificate and key.
	if err := cw.readCertificate(); err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to initialize certwatcher"),
		)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(domain.ErrCertWatcher,
			errors.WithDetail("failed to create filesystem watcher"),
			errors.CausedBy(err),
		)
	}

	cw.watcher = watcher

	return cw, nil
}

// GetCertificate returns the currently loaded certificate. Its signature
// matches tls.Config.GetCertificate so it can be wired in directly.
func (cw *CertWatcher) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return cw.currentCert, nil
}

// Start begins watching the certificate and key files and blocks until ctx is
// cancelled. It returns any error encountered while tearing down the watcher.
func (cw *CertWatcher) Start(ctx context.Context) error {
	if err := cw.addWatches(ctx); err != nil {
		_ = cw.watcher.Close()
		return errors.Wrap(err,
			errors.WithDetail("failed to start certwatcher"),
		)
	}

	go cw.watch(ctx)

	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()

	cw.logger.Debug("starting certificate poll+watcher",
		slog.Duration("interval", cw.interval),
	)

	for {
		select {
		case <-ctx.Done():
			if err := cw.watcher.Close(); err != nil {
				return errors.Wrap(domain.ErrCertWatcher,
					errors.WithDetail("failed to close filesystem watcher"),
					errors.CausedBy(err),
				)
			}

			return nil
		case <-ticker.C:
			if err := cw.readCertificate(); err != nil {
				cw.logger.ErrorContext(ctx, "failed to read certificate",
					slog.Any("error", err),
				)
			}
		}
	}
}

// addWatches registers filesystem watches for the certificate and key files,
// retrying until they can be watched or the timeout elapses. This tolerates the
// files not yet existing when the server starts.
func (cw *CertWatcher) addWatches(ctx context.Context) error {
	pending := map[string]struct{}{
		cw.certPath: {},
		cw.keyPath:  {},
	}

	watchCtx, cancel := context.WithTimeout(ctx, watchAddTimeout)
	defer cancel()

	ticker := time.NewTicker(watchAddInterval)
	defer ticker.Stop()

	var lastErr error

	for {
		for path := range pending {
			if err := cw.watcher.Add(path); err != nil {
				lastErr = err

				continue
			}

			delete(pending, path)
		}

		if len(pending) == 0 {
			return nil
		}

		select {
		case <-watchCtx.Done():
			return errors.Wrap(domain.ErrCertWatcher,
				errors.WithDetail("failed to add file watches after timeout"),
				errors.WithProperty("pending_paths", pending),
				errors.CausedBy(lastErr),
			)
		case <-ticker.C:
		}
	}
}

// watch consumes filesystem events until the watcher is closed.
func (cw *CertWatcher) watch(ctx context.Context) {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			// Channel is closed.
			if !ok {
				return
			}

			cw.handleEvent(ctx, event)
		case err, ok := <-cw.watcher.Errors:
			// Channel is closed.
			if !ok {
				return
			}

			cw.logger.ErrorContext(ctx, "certificate watch error",
				slog.Any("error", err),
			)
		}
	}
}

// handleEvent reacts to a filesystem event, re-watching the file after atomic
// swaps (rename/remove/chmod, as used by Kubernetes secret mounts) and
// re-reading the certificate on any content-affecting change.
func (cw *CertWatcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	switch {
	case event.Op.Has(fsnotify.Write), event.Op.Has(fsnotify.Create):
	case event.Op.Has(fsnotify.Chmod), event.Op.Has(fsnotify.Remove), event.Op.Has(fsnotify.Rename):
		// The file was removed or renamed (e.g. an atomic secret rotation), so
		// re-add the watch on the original path.
		if err := cw.watcher.Add(event.Name); err != nil {
			cw.logger.ErrorContext(ctx, "error re-watching file",
				slog.String("file", event.Name),
				slog.Any("error", err),
			)
		}
	default:
		return
	}

	cw.logger.Debug("certificate event detected",
		slog.String("file", event.Name),
		slog.String("operation", event.Op.String()),
	)

	if err := cw.readCertificate(); err != nil {
		cw.logger.ErrorContext(ctx, "error re-reading certificate",
			slog.Any("error", err),
		)
	}
}

// readCertificate reads and parses the certificate and key files from disk and
// updates the cached certificate if it changed.
func (cw *CertWatcher) readCertificate() error {
	cw.logger.Debug("reading certificate and key files",
		slog.String("cert_path", cw.certPath),
		slog.String("key_path", cw.keyPath),
	)

	certPEMBlock, err := os.ReadFile(cw.certPath)
	if err != nil {
		return errors.Wrap(domain.ErrCertWatcher,
			errors.WithDetail("failed to read TLS certificate file"),
			errors.WithProperty("cert_path", cw.certPath),
			errors.CausedBy(err),
		)
	}

	keyPEMBlock, err := os.ReadFile(cw.keyPath)
	if err != nil {
		return errors.Wrap(domain.ErrCertWatcher,
			errors.WithDetail("failed to read TLS key file"),
			errors.WithProperty("key_path", cw.keyPath),
			errors.CausedBy(err),
		)
	}

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return errors.Wrap(domain.ErrCertWatcher,
			errors.WithDetail("failed to parse TLS certificate and key pair"),
			errors.WithProperty("cert_path", cw.certPath),
			errors.WithProperty("key_path", cw.keyPath),
			errors.CausedBy(err),
		)
	}

	if !cw.updateCachedCertificate(&cert, keyPEMBlock) {
		return nil
	}

	cw.logger.Debug("updated current TLS certificate")

	return nil
}

// updateCachedCertificate swaps in the new certificate when it differs from the
// cached one, returning whether an update occurred.
func (cw *CertWatcher) updateCachedCertificate(cert *tls.Certificate, keyPEMBlock []byte) bool {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.currentCert != nil &&
		bytes.Equal(cw.currentCert.Certificate[0], cert.Certificate[0]) &&
		bytes.Equal(cw.cachedKeyPEMBlock, keyPEMBlock) {
		return false
	}

	cw.currentCert = cert
	cw.cachedKeyPEMBlock = keyPEMBlock

	return true
}
