package di

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scality/static-oci-registry/cmd/config"
)

// TestGetMetricsRESTConfig exercises the two code paths in
// (*Container).getMetricsRESTConfig without spinning up a fake apiserver:
// (a) an explicit METRICS_KUBECONFIG must be loaded and reach the intended
//
//	cluster host, and
//
// (b) an empty METRICS_KUBECONFIG must fall back to InClusterConfig(), which
//
//	returns rest.ErrNotInCluster when executed outside a Kubernetes pod.
func TestGetMetricsRESTConfig(t *testing.T) {
	t.Parallel()

	const kubeconfigYAML = `apiVersion: v1
kind: Config
current-context: test
clusters:
- name: test
  cluster:
    server: https://example.test:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
users:
- name: test
  user:
    token: fake
`

	tmp := t.TempDir()
	kubeconfigPath := filepath.Join(tmp, "kubeconfig")

	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigYAML), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	t.Run("explicit kubeconfig loads to the expected host", func(t *testing.T) {
		t.Parallel()

		c := &Container{config: &config.Environment{Metrics: config.Metrics{Kubeconfig: kubeconfigPath}}}

		restCfg, err := c.getMetricsRESTConfig()
		if err != nil {
			t.Fatalf("getMetricsRESTConfig: %v", err)
		}

		if restCfg.Host != "https://example.test:6443" {
			t.Errorf("Host = %q, want https://example.test:6443", restCfg.Host)
		}
	})

	t.Run("empty kubeconfig falls back to in-cluster config", func(t *testing.T) {
		t.Parallel()

		c := &Container{config: &config.Environment{Metrics: config.Metrics{Kubeconfig: ""}}}

		_, err := c.getMetricsRESTConfig()
		if err == nil {
			t.Fatal("expected InClusterConfig() to fail outside a pod, got nil error")
		}
	})
}
