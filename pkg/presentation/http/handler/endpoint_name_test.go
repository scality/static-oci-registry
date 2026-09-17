package handler_test

import (
	"testing"

	apphttp "github.com/scality/static-oci-registry/pkg/presentation/http"
	"github.com/scality/static-oci-registry/pkg/presentation/http/handler"
)

// TestEndpointNames pins the OCI-spec label of each Route implementation.
// Endpoint labels are part of the Prometheus series identity, so accidental
// renames would silently break dashboards.
func TestEndpointNames(t *testing.T) {
	cases := []struct {
		name  string
		route apphttp.Route
		want  string
	}{
		{"list_tags", &handler.ListTags{}, "list_tags"},
		{"fetch_manifest", &handler.FetchManifest{}, "fetch_manifest"},
		{"pull_blob", &handler.PullBlob{}, "pull_blob"},
		{"unsupported", handler.NewUnsupportedEndpoint(), "unsupported"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.route.EndpointName(); got != tc.want {
				t.Fatalf("EndpointName() = %q, want %q", got, tc.want)
			}
		})
	}
}
