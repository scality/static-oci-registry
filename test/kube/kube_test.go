// Package kube exercises Kubernetes-backed metrics scrape authorization
// against a real kube-apiserver spawned by envtest. envtest boots
// kube-apiserver + etcd as local subprocesses using the binaries under
// KUBEBUILDER_ASSETS (installed via `setup-envtest use <version>`).
package kube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/scality/static-oci-registry/cmd/config"
	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
	"github.com/scality/static-oci-registry/test/utils"
)

const (
	// scrapeTimeout bounds waits on the in-process metrics HTTP server,
	// not envtest boot. envtest boot uses its own default timeouts.
	scrapeTimeout = 5 * time.Second

	// Token identities configured in the token-auth-file consumed by the
	// envtest apiserver. Bearer tokens carried by scrape requests must
	// match one of these to authenticate. Unknown tokens cause the
	// apiserver's TokenReview to return Authenticated:false which the
	// filter surfaces as an authentication error (see scrape_auth_test.go).
	scraperToken      = "scraper-token"
	unauthorizedToken = "unauthorized-token"

	// RBAC identities. Static-token users are authenticated as User
	// (not ServiceAccount), so bindings target Kind: User.
	scraperUser      = "scraper-user"
	unauthorizedUser = "unauth-user"

	metricsReaderRole = "metrics-reader"
)

var (
	ctx            context.Context
	cancel         context.CancelFunc
	testEnv        *envtest.Environment
	metricsAddr    string
	certFilePath   string
	keyFilePath    string
	tempDir        string
	metricsSrv     *http.Server
	insecureClient *http.Client
	kubeconfigPath string
)

func TestKube(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kube Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	tempDir = GinkgoT().TempDir()

	// Static-token file consumed by the apiserver's --token-auth-file
	// authenticator. Format is `token,user,uid[,"groups"]`; we omit the
	// groups column since only per-user RBAC bindings are needed. This
	// file MUST exist before envtest.Start reads it at apiserver boot.
	tokenFilePath := filepath.Join(tempDir, "tokens.csv")
	csv := scraperToken + "," + scraperUser + ",scraper-uid\n" +
		unauthorizedToken + "," + unauthorizedUser + ",unauth-uid\n"
	Expect(os.WriteFile(tokenFilePath, []byte(csv), 0o600)).To(Succeed())

	testEnv = &envtest.Environment{}
	testEnv.ControlPlane.APIServer = &envtest.APIServer{}
	testEnv.ControlPlane.APIServer.Configure().Append("token-auth-file", tokenFilePath)

	restCfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	installRBAC(restCfg)

	// Persist the admin kubeconfig envtest generated so metrics_auth's
	// DI wiring can build its own REST client from a real file, exactly
	// like production. The admin identity is system:masters, which has
	// blanket permissions to create TokenReviews and SubjectAccessReviews.
	kubeconfigPath = filepath.Join(tempDir, "kubeconfig")
	Expect(os.WriteFile(kubeconfigPath, testEnv.KubeConfig, 0o600)).To(Succeed())

	certFilePath, keyFilePath = utils.GenerateSelfSignedCert(tempDir)

	cfg := newTestConfig(kubeconfigPath, certFilePath, keyFilePath, true)
	container := di.NewContainer(ctx, cfg)

	container.GetHTTPServer()
	seedRequestMetric(container.GetHTTPServer().Handler)

	go func() {
		defer GinkgoRecover()

		Expect(container.GetHTTPCertWatcher().Start(ctx)).To(Succeed())
	}()
	go func() {
		defer GinkgoRecover()

		Expect(container.GetMetricsCertWatcher().Start(ctx)).To(Succeed())
	}()

	metricsSrv, metricsAddr = startMetricsServer(container.GetMetricsServer(), true)
	insecureClient = newMetricsClient(certFilePath)

	Expect(waitForAnyResponse(insecureClient, metricsAddr+"/metrics", scrapeTimeout)).To(BeTrue())
})

var _ = AfterSuite(func() {
	cancel()

	if metricsSrv != nil {
		_ = metricsSrv.Shutdown(context.Background())
	}

	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})

// installRBAC provisions the ClusterRole granting read access to the
// non-resource /metrics endpoint plus a ClusterRoleBinding to the scraper
// user. The unauthorized user is intentionally left unbound so the
// apiserver's SubjectAccessReview returns Allowed: false for it.
func installRBAC(restCfg *rest.Config) {
	clientset, err := kubernetes.NewForConfig(restCfg)
	Expect(err).NotTo(HaveOccurred())

	_, err = clientset.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: metricsReaderRole},
		Rules: []rbacv1.PolicyRule{{
			NonResourceURLs: []string{"/metrics"},
			Verbs:           []string{"get"},
		}},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	_, err = clientset.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "metrics-scraper"},
		Subjects: []rbacv1.Subject{{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "User",
			Name:     scraperUser,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     metricsReaderRole,
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func newTestConfig(kubeconfig, certFile, keyFile string, secure bool) *config.Environment {
	cfg, err := config.NewEnvironment(ctx)
	Expect(err).NotTo(HaveOccurred())

	cfg.LogLevel = "error"
	cfg.FS.Root = tempDir
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.HTTP.TLS.CertFilePath = certFile
	cfg.HTTP.TLS.KeyFilePath = keyFile
	cfg.Metrics.Addr = "127.0.0.1:0"
	cfg.Metrics.Secure = secure
	cfg.Metrics.Kubeconfig = kubeconfig
	cfg.Metrics.TLS.CertFilePath = certFile
	cfg.Metrics.TLS.KeyFilePath = keyFile
	cfg.Registry.Name = "kube-test"

	return cfg
}

func startMetricsServer(server *http.Server, secure bool) (*http.Server, string) {
	Expect(server).NotTo(BeNil())

	listener, err := net.Listen("tcp", server.Addr)
	Expect(err).NotTo(HaveOccurred())

	addr := listener.Addr().String()

	scheme := "http"
	if secure {
		scheme = "https"
	}

	go func() {
		defer GinkgoRecover()

		var serveErr error
		if secure {
			serveErr = server.ServeTLS(listener, "", "")
		} else {
			serveErr = server.Serve(listener)
		}

		Expect(serveErr).To(MatchError(http.ErrServerClosed))
	}()

	return server, scheme + "://" + addr
}

func newMetricsClient(caPath string) *http.Client {
	certPEM, err := os.ReadFile(caPath)
	Expect(err).NotTo(HaveOccurred())

	rootCAs := x509.NewCertPool()
	Expect(rootCAs.AppendCertsFromPEM(certPEM)).To(BeTrue())

	return &http.Client{
		Timeout: scrapeTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    rootCAs,
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

func waitForAnyResponse(client *http.Client, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()

			return true
		}

		time.Sleep(100 * time.Millisecond)
	}

	return false
}

func getMetrics(client *http.Client, addr, bearer string) (*http.Response, []byte) {
	req, err := http.NewRequest(http.MethodGet, addr+"/metrics", nil)
	Expect(err).NotTo(HaveOccurred())

	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return resp, body
}

func shutdownServer(server *http.Server) {
	Expect(server.Shutdown(context.Background())).To(Succeed())
}

func seedRequestMetric(handler http.Handler) {
	req := httptest.NewRequest(http.MethodGet, "https://registry.test/v2/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
}
