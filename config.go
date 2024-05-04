package envoy

import (
	"errors"
	"sync"

	"github.com/ardikabs/go-envoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Configuration interface {
	GetFilterConfig() interface{}
	GetConfigCallbacks() api.ConfigCallbacks

	// Store allows you to save a value of any type under a key of any type,
	// It designed for sharing data throughout the lifetime of Envoy.
	//
	// Please be cautious! The Store function overwrites any existing data.
	Store(key any, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	// It designed for sharing data throughout the lifetime of Envoy.
	//
	// It returns true if a compatible value is successfully loaded,
	// and false if no value is found or an error occurs during the process.
	Load(key any, receiver interface{}) (ok bool, err error)

	metricCounter(name string) api.CounterMetric
	metricGauge(name string) api.GaugeMetric
	metricHistogram(name string) api.HistogramMetric
}

type config struct {
	filterCfg interface{}
	callbacks api.ConfigCallbacks

	gaugeMetric     map[string]api.GaugeMetric
	counterMetric   map[string]api.CounterMetric
	histogramMetric map[string]api.HistogramMetric

	storage *sync.Map
}

func newConfig(filterCfg interface{}, cc api.ConfigCallbacks) *config {
	return &config{
		filterCfg:       filterCfg,
		callbacks:       cc,
		gaugeMetric:     make(map[string]api.GaugeMetric),
		counterMetric:   make(map[string]api.CounterMetric),
		histogramMetric: make(map[string]api.HistogramMetric),
		storage:         new(sync.Map),
	}
}

func (c *config) GetFilterConfig() interface{} {
	return c.filterCfg
}

func (c *config) GetConfigCallbacks() api.ConfigCallbacks {
	return c.callbacks
}

func (c *config) metricCounter(name string) api.CounterMetric {
	counter, ok := c.counterMetric[name]
	if !ok {
		counter = c.callbacks.DefineCounterMetric(name)
		c.counterMetric[name] = counter
	}
	return counter
}

func (c *config) metricGauge(name string) api.GaugeMetric {
	gauge, ok := c.gaugeMetric[name]
	if !ok {
		gauge = c.callbacks.DefineGaugeMetric(name)
		c.gaugeMetric[name] = gauge
	}
	return gauge
}

func (c *config) metricHistogram(name string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

func (c *config) Store(key any, value any) {
	c.storage.Store(key, value)
}

func (c *config) Load(key any, receiver interface{}) (bool, error) {
	if receiver == nil {
		return false, errors.New("config: receiver should not be nil")
	}

	v, ok := c.storage.Load(key)
	if !ok {
		return false, nil
	}

	if !util.CastTo(receiver, v) {
		return false, errors.New("config: receiver and value has an incompatible type")
	}

	return true, nil
}
