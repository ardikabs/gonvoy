package gonvoy

import (
	"errors"
	"strings"
	"sync"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type Configuration interface {
	// GetFilterConfig returns the filter configuration associated with the route.
	// It defaults to the parent filter configuration if no route filter configuration was found.
	// Otherwise, once typed_per_filter_config present in the route then it will return the child filter configuration.
	// Whether these filter configurations can be merged depends on the filter configuration struct tags.
	//
	GetFilterConfig() interface{}

	// Cache provides the global cache, that persists throughout Envoy's lifespan.
	// Use this cache when variable initialization is expensive or requires a statefulness.
	//
	Cache() Cache
}

type globalConfig struct {
	filterCfg interface{}

	callbacks  api.ConfigCallbacks
	localCache Cache

	metricPrefix string
	gaugeMap     map[string]api.GaugeMetric
	counterMap   map[string]api.CounterMetric
	histogramMap map[string]api.HistogramMetric

	disabledHttpFilterPhases []HttpFilterPhase
}

func newGlobalConfig(cc api.ConfigCallbacks, options ConfigOptions) *globalConfig {
	gc := &globalConfig{
		callbacks:                cc,
		localCache:               NewCache(),
		gaugeMap:                 make(map[string]api.GaugeMetric),
		counterMap:               make(map[string]api.CounterMetric),
		histogramMap:             make(map[string]api.HistogramMetric),
		disabledHttpFilterPhases: options.DisabledHttpFilterPhases,
		metricPrefix:             options.MetricPrefix,
	}

	return gc

}

func (c *globalConfig) GetFilterConfig() interface{} {
	return c.filterCfg
}

func (c *globalConfig) Cache() Cache {
	return c.localCache
}

func (c *globalConfig) metricCounter(name string) api.CounterMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricPrefix + name))
	counter, ok := c.counterMap[name]
	if !ok {
		counter = c.callbacks.DefineCounterMetric(name)
		c.counterMap[name] = counter
	}
	return counter
}

func (c *globalConfig) metricGauge(name string) api.GaugeMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricPrefix + name))
	gauge, ok := c.gaugeMap[name]
	if !ok {
		gauge = c.callbacks.DefineGaugeMetric(name)
		c.gaugeMap[name] = gauge
	}
	return gauge
}

func (c *globalConfig) metricHistogram(name string) api.HistogramMetric {
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
