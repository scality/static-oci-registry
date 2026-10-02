package utils

import (
	"bytes"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	. "github.com/onsi/gomega" //nolint:revive,staticcheck // only gomega and ginkgo are to be used as dot imports
)

// ParseMetricsText decodes a Prometheus text-format response body into a map
// keyed by metric family name.
func ParseMetricsText(body []byte) map[string]*dto.MetricFamily {
	parser := expfmt.NewTextParser(model.UTF8Validation)

	families, err := parser.TextToMetricFamilies(bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())

	return families
}

// FindCounter returns the value of the counter series in fam whose labels are a
// superset of want. It returns 0, false when no matching series exists.
func FindCounter(fam *dto.MetricFamily, want map[string]string) (float64, bool) {
	for _, metric := range fam.GetMetric() {
		if labelsInclude(metric.GetLabel(), want) {
			return metric.GetCounter().GetValue(), true
		}
	}

	return 0, false
}

// FindHistogramCount returns the _count of the histogram series in fam whose
// labels are a superset of want. It returns 0, false when no matching series
// exists.
func FindHistogramCount(fam *dto.MetricFamily, want map[string]string) (uint64, bool) {
	for _, metric := range fam.GetMetric() {
		if labelsInclude(metric.GetLabel(), want) {
			return metric.GetHistogram().GetSampleCount(), true
		}
	}

	return 0, false
}

func labelsInclude(labels []*dto.LabelPair, want map[string]string) bool {
	got := make(map[string]string, len(labels))
	for _, label := range labels {
		got[label.GetName()] = label.GetValue()
	}

	for name, value := range want {
		if got[name] != value {
			return false
		}
	}

	return true
}
