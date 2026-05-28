package integration

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/domain/ocierrors"
	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
	apphttp "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/test/utils"
)

type QueryParams map[string]string

const timeoutDurationInSeconds = 5

var (
	suite  *utils.TestSuite
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Environment

	httpServer *http.Server
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

// nolint: unparam // This param will have different values in the future
func initRequest(image, path string, params QueryParams) *http.Request {
	return initRequestWithMethod(http.MethodGet, image, path, params)
}

func initRequestWithMethod(method, image, path string, params QueryParams) *http.Request {
	sanitizedImage := strings.ReplaceAll(image, "/", "%2F")
	url := "https://localhost" + cfg.HTTP.Addr + "/v2/" + sanitizedImage + path
	req, err := http.NewRequest(method, url, nil)
	Expect(err).NotTo(HaveOccurred())

	q := req.URL.Query()
	for key, value := range params {
		q.Set(key, value)
	}

	req.URL.RawQuery = q.Encode()

	return req
}

// execRequestRaw executes the request and returns the response body verbatim.
// Required for endpoints whose body is content-addressed (e.g. manifests),
// where every byte is significant to the Docker-Content-Digest header.
func execRequestRaw(client *http.Client, req *http.Request) (*http.Response, []byte) {
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, body
}

// execRequest executes the request and strips the trailing newline that
// json.Encoder appends to JSON responses. Use for endpoints that respond
// via RespondWithJSON; use execRequestRaw for verbatim bodies.
func execRequest(client *http.Client, req *http.Request) (*http.Response, []byte) {
	resp, body := execRequestRaw(client, req)

	return resp, body[:len(body)-1]
}

func checkErrorResponse(body []byte, code ocierrors.OCIErrorCode) {
	errorResponse := apphttp.NewErrorResponse()

	Expect(json.Unmarshal(body, errorResponse)).To(Succeed(),
		"response body is not a valid OCI error JSON, found: %s", body)

	Expect(errorResponse.Errors).To(ContainElement(HaveField("Code", code)))
}

var _ = BeforeSuite(func() {
	suite = utils.NewTestSuiteNoLogger("integration")

	// init context
	ctx, cancel = context.WithTimeout(context.Background(), timeoutDurationInSeconds*time.Second)

	// init env
	var err error

	cfg, err = config.NewEnvironment(ctx)
	Expect(err).NotTo(HaveOccurred())

	// fill cfg values
	cfg.FS.Root = suite.FsRoot
	cfg.LogLevel = "error"

	// generate a self-signed cert for TLS
	cfg.HTTP.TLS.CertFilePath, cfg.HTTP.TLS.KeyFilePath = utils.GenerateSelfSignedCert(suite.FsRoot)

	// init di containter
	container := di.NewContainer(ctx, cfg)

	// get logger from container
	suite.Logger = container.GetLogger()

	// start https server
	httpServer = container.GetHTTPServer()

	go func() {
		serveErr := httpServer.ListenAndServeTLS("", "")
		Expect(serveErr).To(MatchError(http.ErrServerClosed))
	}()

	insecureClient := &http.Client{
		Timeout: timeoutDurationInSeconds * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed cert in tests
		},
	}

	Expect(
		waitForServer(
			insecureClient,
			"https://localhost"+cfg.HTTP.Addr+"/v2/",
			timeoutDurationInSeconds*time.Second,
		),
	).To(BeTrue())

	req, err := http.NewRequest(http.MethodGet, "https://localhost"+cfg.HTTP.Addr+"/v2/", nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := insecureClient.Do(req)
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	Expect(resp.StatusCode).To(Equal(http.StatusOK))
})

var _ = AfterSuite(func() {
	cancel()

	defer suite.CleanupFsRoot()

	err := httpServer.Shutdown(ctx)
	Expect(err).NotTo(HaveOccurred())
})

func waitForServer(client *http.Client, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}

		time.Sleep(100 * time.Millisecond)
	}

	return false
}
