package e2e

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const timeoutDurationInSeconds = 5

var (
	registryEndpoint string
	client           *http.Client
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

// this test assumes that the registry is already running.
var _ = BeforeSuite(func() {
	registryHost := os.Getenv("TEST_REGISTRY_HOST")
	if registryHost == "" {
		registryHost = "localhost"
	}

	registryPort := os.Getenv("TEST_REGISTRY_PORT")
	if registryPort == "" {
		registryPort = "5000"
	}

	registryEndpoint = registryHost + ":" + registryPort

	client = &http.Client{
		Timeout: timeoutDurationInSeconds * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Check if the registry is up and running by sending a request to the /v2/ endpoint
	Eventually(func() error {
		resp, err := client.Get("https://" + registryEndpoint + "/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}

		return nil
	}, timeoutDurationInSeconds*time.Second, time.Second).Should(Succeed())
})
