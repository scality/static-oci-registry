package reqlabels_test

import (
	"context"
	"testing"

	"github.com/scality/static-oci-registry/pkg/domain"
	"github.com/scality/static-oci-registry/pkg/presentation/http/reqlabels"
)

func TestNewContext_installsMutableBag(t *testing.T) {
	ctx, labels := reqlabels.NewContext(context.Background())

	if labels == nil {
		t.Fatal("expected non-nil labels bag")
	}

	if got := reqlabels.From(ctx); got != labels {
		t.Fatalf("From(ctx) returned a different pointer than NewContext: got %p want %p", got, labels)
	}
}

func TestFrom_missingBagReturnsNil(t *testing.T) {
	if got := reqlabels.From(context.Background()); got != nil {
		t.Fatalf("From on plain context: got %v want nil", got)
	}

	if got := reqlabels.From(nil); got != nil { //nolint:staticcheck // exercising the nil-ctx branch on purpose
		t.Fatalf("From(nil): got %v want nil", got)
	}
}

func TestSetters_areNilSafe(t *testing.T) {
	var nilBag *reqlabels.RequestLabels

	// Must not panic.
	nilBag.SetEndpoint("list_tags")
	nilBag.SetSolution(domain.SolutionVersion{Solution: "sol", Version: "1.0.0"})
}

func TestSetters_mutateInPlace(t *testing.T) {
	ctx, labels := reqlabels.NewContext(context.Background())

	labels.SetEndpoint("pull_blob")
	reqlabels.From(ctx).SetSolution(domain.SolutionVersion{Solution: "acme", Version: "2.3.4"})

	want := reqlabels.RequestLabels{
		Endpoint:        "pull_blob",
		SolutionName:    "acme",
		SolutionVersion: "2.3.4",
	}

	if *labels != want {
		t.Fatalf("labels = %+v, want %+v", *labels, want)
	}
}
