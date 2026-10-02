// Package kube contains metrics scrape auth test cases.
package kube

import (
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/static-oci-registry/pkg/infrastructure/di"
)

var _ = Describe("Metrics scrape auth", func() {
	It("returns 401 when no Authorization header is set", func() {
		resp, _ := getMetrics(insecureClient, metricsAddr, "")

		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	})

	It("returns 500 when the bearer token is unknown", func() {
		// The apiserver's static-token authenticator returns
		// TokenReview{Status: {Authenticated: false}} for tokens absent
		// from --token-auth-file. The controller-runtime filter wraps a
		// bearertoken authenticator (k8s.io/apiserver/.../bearertoken)
		// that synthesizes `errors.New("invalid bearer token")` for the
		// (nil, false, nil) webhook result, and the filter maps err !=
		// nil to HTTP 500. Only requests with no Authorization header at
		// all reach the 401 branch above.
		resp, _ := getMetrics(insecureClient, metricsAddr, "completely-bogus")

		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
	})

	It("returns 403 when the caller is authenticated but not authorized", func() {
		resp, _ := getMetrics(insecureClient, metricsAddr, unauthorizedToken)

		Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
	})

	It("returns 200 and metrics text when the caller is fully authorized", func() {
		resp, body := getMetrics(insecureClient, metricsAddr, scraperToken)

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get("Content-Type")).To(HavePrefix("text/plain"))
		Expect(string(body)).To(ContainSubstring("# HELP registry_http_requests_total"))
	})

	Context("with Metrics.Secure=false", Ordered, func() {
		var (
			insecureMetricsSrv  *http.Server
			insecureMetricsAddr string
		)

		BeforeAll(func() {
			cfg := newTestConfig("", certFilePath, keyFilePath, false)
			container := di.NewContainer(ctx, cfg)

			container.GetHTTPServer()
			seedRequestMetric(container.GetHTTPServer().Handler)
			insecureMetricsSrv, insecureMetricsAddr = startMetricsServer(container.GetMetricsServer(), false)

			Expect(waitForAnyResponse(insecureClient, insecureMetricsAddr+"/metrics", scrapeTimeout)).To(BeTrue())
		})

		AfterAll(func() {
			shutdownServer(insecureMetricsSrv)
		})

		It("serves /metrics without any Authorization header", func() {
			resp, body := getMetrics(insecureClient, insecureMetricsAddr, "")

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(strings.TrimSpace(string(body))).To(ContainSubstring("registry_http_requests_total"))
		})
	})
})
