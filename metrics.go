package gonvoy

import (
	"fmt"
	"strings"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// Metrics is an interface for defining various types of metrics.
// At presents, it only supports Gauge and Counter.
type Metrics interface {
	// Gauge sets gauge statistics that can record both increasing and decreasing metrics. E.g., current active requests.
	Gauge(name string, labelKeyValues ...string) api.GaugeMetric

	// Counter sets counter statistics that only record for increase, but never decrease metrics. E.g., total requests.
	Counter(name string, labelKeyValues ...string) api.CounterMetric

	// Histogram is still a WIP
	Histogram(name string, labelKeyValues ...string) api.HistogramMetric
}

// newMetrics builds the per-stream recorder. Handles are memoized in the
// shared config cache, so no per-stream maps are kept: a stream normally
// serves one request and a lookup cache that is written once and never read
// again is pure overhead.
func newMetrics(counterFunc counterFunc, gaugeFunc gaugeFunc, histogramFunc histogramFunc) *metrics {
	return &metrics{
		counterFunc:   counterFunc,
		gaugeFunc:     gaugeFunc,
		histogramFunc: histogramFunc,
	}
}

var _ Metrics = &metrics{}

type (
	counterFunc   func(name string) api.CounterMetric
	gaugeFunc     func(name string) api.GaugeMetric
	histogramFunc func(name string) api.HistogramMetric

	metrics struct {
		counterFunc   counterFunc
		gaugeFunc     gaugeFunc
		histogramFunc histogramFunc
	}
)

func (m *metrics) Gauge(name string, labelKeyValues ...string) api.GaugeMetric {
	if m.gaugeFunc == nil {
		panic("metric gauge handler is not set")
	}

	return m.gaugeFunc(createStatsName(name, labelKeyValues...))
}

func (m *metrics) Counter(name string, labelKeyValues ...string) api.CounterMetric {
	if m.counterFunc == nil {
		panic("metric counter handler is not set")
	}

	return m.counterFunc(createStatsName(name, labelKeyValues...))
}

func (m *metrics) Histogram(name string, labelKeyValues ...string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

// Create an Envoy stats name with the given name and labels.
func createStatsName(name string, labels ...string) string {
	if len(labels) == 0 {
		return name + "_"
	}
	if len(labels)%2 != 0 {
		return name + "_bad_labels"
	}

	var fmtLabels []string
	for i := 0; i < len(labels); i += 2 {
		fmtLabels = append(fmtLabels, fmt.Sprintf("%s=%s", strings.ReplaceAll(labels[i], "-", "_"), labels[i+1]))
	}

	return name + "_" + strings.Join(fmtLabels, "_")
}
