package gonvoy

import (
	"errors"
	"sync"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Configuration interface {
	GetFilterConfig() interface{}
	GetConfigCallbacks() api.ConfigCallbacks
	Cache() Cache

	metricCounter(name string) api.CounterMetric
	metricGauge(name string) api.GaugeMetric
	metricHistogram(name string) api.HistogramMetric
}

type config struct {
	filterCfg  interface{}
	callbacks  api.ConfigCallbacks
	localCache Cache

	gaugeMetric     map[string]api.GaugeMetric
	counterMetric   map[string]api.CounterMetric
	histogramMetric map[string]api.HistogramMetric
}

func newConfig(filterCfg interface{}, cc api.ConfigCallbacks) *config {
	return &config{
		filterCfg:       filterCfg,
		callbacks:       cc,
		localCache:      NewCache(),
		gaugeMetric:     make(map[string]api.GaugeMetric),
		counterMetric:   make(map[string]api.CounterMetric),
		histogramMetric: make(map[string]api.HistogramMetric),
	}
}

func (c *config) GetFilterConfig() interface{} {
	return c.filterCfg
}

func (c *config) GetConfigCallbacks() api.ConfigCallbacks {
	return c.callbacks
}

func (c *config) Cache() Cache {
	return c.localCache
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

type Cache interface {
	// Store allows you to save a value of any type under a key of any type,
	// It designed for sharing data throughout Envoy's lifespan.
	//
	// Please be cautious! The Store function overwrites any existing data.
	Store(key, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	// It designed for sharing data throughout Envoy's lifespan..
	//
	// It returns true if a compatible value is successfully loaded,
	// and false if no value is found or an error occurs during the process.
	Load(key any, receiver interface{}) (ok bool, err error)
}

type localcache struct {
	dataMap sync.Map
}

func NewCache() *localcache {
	return &localcache{}
}

func (c *localcache) Store(key, value any) {
	c.dataMap.Store(key, value)
}

func (c *localcache) Load(key any, receiver interface{}) (bool, error) {
	if receiver == nil {
		return false, errors.New("receiver should not be nil")
	}

	v, ok := c.dataMap.Load(key)
	if !ok {
		return false, nil
	}

	if !util.CastTo(receiver, v) {
		return false, errors.New("receiver and value has an incompatible type")
	}

	return true, nil
}
