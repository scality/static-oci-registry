package integration

import (
	"context"
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

func initRequest(image, path string, params QueryParams) *http.Request {
	sanitizedImage := strings.ReplaceAll(string(image), "/", "%2F")
	url := "http://localhost" + cfg.HTTP.Addr + "/v2/" + sanitizedImage + path
	req, err := http.NewRequest("GET", url, nil)
	Expect(err).NotTo(HaveOccurred())

	q := req.URL.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()

	return req
}

func execRequest(client *http.Client, req *http.Request) (*http.Response, []byte) {
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, body
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

	// init di containter
	container := di.NewContainer(ctx, cfg)

	// get logger from container
	suite.Logger = container.GetLogger()

	// start http server
	httpServer = container.GetHTTPServer()

	go func () {
		serveErr := httpServer.ListenAndServe()
		Expect(serveErr).To(MatchError(http.ErrServerClosed))
	}()

	// maybe wait for server to be ready
	client := &http.Client{
		Timeout: timeoutDurationInSeconds * time.Second,
	}
	req, err := http.NewRequest("GET", "http://localhost"+cfg.HTTP.Addr+"/v2/", nil)
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.Do(req)
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
