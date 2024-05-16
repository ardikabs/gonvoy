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

func newMetrics(counterFunc counterFunc, gaugeFunc gaugeFunc, histogramFunc histogramFunc) *metrics {
	return &metrics{
		counter:   counterFunc,
		gauge:     gaugeFunc,
		histogram: histogramFunc,
	}
}

var _ Metrics = &metrics{}

type (
	counterFunc   func(name string) api.CounterMetric
	gaugeFunc     func(name string) api.GaugeMetric
	histogramFunc func(name string) api.HistogramMetric

	metrics struct {
		counter   counterFunc
		gauge     gaugeFunc
		histogram histogramFunc
	}
)

func (m *metrics) Gauge(name string, labelKeyValues ...string) api.GaugeMetric {
	fqn := fmt.Sprintf("%s_%s", name, formatMetricLabels(labelKeyValues...))
	return m.gauge(fqn)
}

func (m *metrics) Counter(name string, labelKeyValues ...string) api.CounterMetric {
	fqn := fmt.Sprintf("%s_%s", name, formatMetricLabels(labelKeyValues...))
	return m.counter(fqn)
}

func (m *metrics) Histogram(name string, labelKeyValues ...string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

func formatMetricLabels(rawLabels ...string) string {
	if len(rawLabels)%2 != 0 {
		return ""
	}

	var fmtLabels []string
	for i := 0; i < len(rawLabels); i += 2 {
		fmtLabels = append(fmtLabels, fmt.Sprintf("%s=%s", strings.Replace(rawLabels[i], "-", "_", -1), rawLabels[i+1]))
	}

	return strings.Join(fmtLabels, "_")
}
