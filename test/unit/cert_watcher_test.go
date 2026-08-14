package unit

import (
	"context"
	"crypto/x509"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	// nolint: revive,staticcheck // only gomega and ginkgo are to be used as dot imports
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/pkg/infrastructure/certwatcher"
	"github.com/scality/static-oci-registry/test/utils"
)

var _ = Describe("CertWatcher", func() {
	var (
		dir      string
		certPath string
		keyPath  string
		logger   *slog.Logger
	)

	// certSerial returns the serial number of the certificate currently served
	// by the watcher, which WriteSelfSignedCert sets so reloads are observable.
	certSerial := func(cw *certwatcher.CertWatcher) int64 {
		cert, err := cw.GetCertificate(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(cert).NotTo(BeNil())
		Expect(cert.Certificate).NotTo(BeEmpty())

		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		Expect(err).NotTo(HaveOccurred())

		return leaf.SerialNumber.Int64()
	}

	// startWatcher runs the watcher in the background and returns a stop function
	// that cancels it and waits for it to exit cleanly.
	startWatcher := func(cw *certwatcher.CertWatcher) func() {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() { done <- cw.Start(ctx) }()

		// Give Start time to establish its filesystem watches before the test
		// mutates the files, otherwise the change events could be missed.
		time.Sleep(200 * time.Millisecond)

		return func() {
			cancel()
			Eventually(done).Should(Receive(BeNil()))
		}
	}

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		certPath = filepath.Join(dir, "tls.crt")
		keyPath = filepath.Join(dir, "tls.key")
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	})

	It("loads the certificate at construction", func() {
		utils.WriteSelfSignedCert(certPath, keyPath, 1)

		cw, err := certwatcher.New(logger, certPath, keyPath)
		Expect(err).NotTo(HaveOccurred())

		Expect(certSerial(cw)).To(Equal(int64(1)))
	})

	It("returns an error when the certificate files are missing", func() {
		cw, err := certwatcher.New(logger, certPath, keyPath)
		Expect(err).To(HaveOccurred())
		Expect(cw).To(BeNil())
	})

	It("reloads the certificate when the files change on disk", func() {
		utils.WriteSelfSignedCert(certPath, keyPath, 1)

		cw, err := certwatcher.New(logger, certPath, keyPath)
		Expect(err).NotTo(HaveOccurred())

		stop := startWatcher(cw)
		defer stop()

		// Rotate the certificate on disk; the watcher should pick it up.
		utils.WriteSelfSignedCert(certPath, keyPath, 2)

		Eventually(func() int64 { return certSerial(cw) }, "10s", "100ms").Should(Equal(int64(2)))
	})

	It("keeps serving the last good certificate when the files become invalid", func() {
		utils.WriteSelfSignedCert(certPath, keyPath, 1)

		cw, err := certwatcher.New(logger, certPath, keyPath)
		Expect(err).NotTo(HaveOccurred())

		stop := startWatcher(cw)
		defer stop()

		// A corrupt certificate file must not clobber the cached certificate.
		Expect(os.WriteFile(certPath, []byte("not a certificate"), 0o600)).To(Succeed())

		Consistently(func() int64 { return certSerial(cw) }, "1s", "100ms").Should(Equal(int64(1)))
	})
})
