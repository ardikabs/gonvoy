package gonvoy

import (
	"fmt"
	"strings"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Metrics interface {
	// Gauge sets gauge statistics that can record both increasing and decreasing metrics. E.g., current active requests.
	Gauge(name string, labelKeyValues ...string) api.GaugeMetric
	// Counter sets counter statistics that only record for increase, but never decrease metrics. E.g., total requests.
	Counter(name string, labelKeyValues ...string) api.CounterMetric
	// Histogram is still a WIP
	Histogram(name string, labelKeyValues ...string) api.HistogramMetric
}

var _ Metrics = &metrics{}

type metrics struct {
	config Configuration
}

func NewMetrics(config Configuration) Metrics {
	return &metrics{config}
}

func (m *metrics) Gauge(name string, labelKeyValues ...string) api.GaugeMetric {
	fqn := fmt.Sprintf("%s_%s", name, formatMetricLabels(labelKeyValues...))
	return m.config.metricGauge(fqn)
}

func (m *metrics) Counter(name string, labelKeyValues ...string) api.CounterMetric {
	fqn := fmt.Sprintf("%s_%s", name, formatMetricLabels(labelKeyValues...))
	return m.config.metricCounter(fqn)
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
