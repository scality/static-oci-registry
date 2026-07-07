package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"

	. "github.com/onsi/gomega" //nolint:revive,staticcheck // only gomega and ginkgo are to be used as dot imports
)

// GenerateSelfSignedCert writes a self-signed TLS certificate and key to files
// in the given directory and returns their paths.
func GenerateSelfSignedCert(dir string) (certFile, keyFile string) {
	certFile = dir + "/cert.pem"
	keyFile = dir + "/key.pem"

	WriteSelfSignedCert(certFile, keyFile, 1)

	return certFile, keyFile
}

// WriteSelfSignedCert writes a fresh self-signed TLS certificate and key to the
// given paths, using serial as the certificate serial number so that callers
// can generate distinguishable certificates (e.g. to exercise a reload).
func WriteSelfSignedCert(certFile, keyFile string, serial int64) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	certOut, err := os.Create(certFile)
	Expect(err).NotTo(HaveOccurred())

	defer certOut.Close()

	Expect(pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})).To(Succeed())

	keyOut, err := os.Create(keyFile)
	Expect(err).NotTo(HaveOccurred())

	defer keyOut.Close()

	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).NotTo(HaveOccurred())

	Expect(pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})).To(Succeed())
}
