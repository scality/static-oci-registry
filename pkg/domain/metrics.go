package domain

const (
	MetricsNamespace = "registry"
	MetricsSubsystem = "http"

	// LabelMethod and LabelCode are mandatory by promhttp.InstrumentHandlerDuration
	// and cannot be changed.
	LabelMethod          = "method"
	LabelCode            = "code"
	LabelEndpoint        = "endpoint"
	LabelSolutionName    = "solution_name"
	LabelSolutionVersion = "solution_version"
	LabelRegistry        = "registry"

	// EndpointUnknown is the value written to the `endpoint` label when a /v2/
	// request matches no route. Emitting a fixed sentinel (instead of the empty
	// string) keeps the observation countable while bounding cardinality: every
	// unmatched path folds into a single series rather than one per URL.
	EndpointUnknown = "unknown"
)

// RequestDurationBuckets are tuned for a registry serving blobs whose sizes
// span kilobyte manifests to multi-gigabyte layers. The tail extends to 30
// minutes so long pulls do not silently fall into the +Inf bucket.
//
//nolint:gochecknoglobals // fixed histogram layout is intentional
var RequestDurationBuckets = []float64{
	0.005, 0.025, 0.1, 0.25, 1, 5, 30, 120, 600, 1800,
}

// MetricsLabels is the common label set carried by both request vecs.
// Order is not significant to Prometheus but the shared slice keeps
// counter and histogram in lockstep.
//
//nolint:gochecknoglobals // fixed label schema is intentional
var MetricsLabels = []string{
	LabelMethod,
	LabelCode,
	LabelEndpoint,
	LabelSolutionName,
	LabelSolutionVersion,
	LabelRegistry,
}
