package gonvoy

import (
	"strings"
	"sync"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type globalConfig struct {
	filterConfig interface{}

	callbacks   api.ConfigCallbacks
	globalCache Cache

	metricPrefix string
	gaugeMap     map[string]api.GaugeMetric
	counterMap   map[string]api.CounterMetric
	histogramMap map[string]api.HistogramMetric

	strictBodyAccess       bool
	allowRequestBodyRead   bool
	allowRequestBodyWrite  bool
	allowResponseBodyRead  bool
	allowResponseBodyWrite bool
}

func newGlobalConfig(cc api.ConfigCallbacks, options ConfigOptions) *globalConfig {
	gc := &globalConfig{
		callbacks:    cc,
		globalCache:  newCache(),
		gaugeMap:     make(map[string]api.GaugeMetric),
		counterMap:   make(map[string]api.CounterMetric),
		histogramMap: make(map[string]api.HistogramMetric),
		metricPrefix: options.MetricPrefix,

		strictBodyAccess:       !options.DisableStrictBodyAccess,
		allowRequestBodyRead:   options.EnableRequestBodyRead,
		allowRequestBodyWrite:  options.EnableRequestBodyWrite,
		allowResponseBodyRead:  options.EnableResponseBodyRead,
		allowResponseBodyWrite: options.EnableResponseBodyWrite,
	}

	return gc

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

// Cache is an interface that provides methods for accessing an internal cache.
// There are two types of cache implementations supported by the framework:
// - Local cache: A cache that is specific to each HTTP context flow and can be accessed using Context.LocalCache().
// - Global cache: A cache that is shared across all HTTP contexts or throughout the lifespan of Envoy. It can be accessed using Context.GlobalCache().
type Cache interface {
	// Store allows you to save a value of any type under a key of any type.
	//
	// Please use caution! The Store function overwrites any existing data.
	Store(key, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	//
	// It returns true if a compatible value is successfully loaded,
	// false if no value is found, or an error occurs during the process.
	//
	// If the receiver is not a pointer to the stored data type,
	// Load will return an ErrIncompatibleReceiver.
	//
	// Example usage:
	//   type mystruct struct{}
	//
	//   data := new(mystruct)
	//   cache.Store("keyName", data)
	//
	//   receiver := new(mystruct)
	//   _, _ = cache.Load("keyName", &receiver)
	Load(key, receiver any) (ok bool, err error)
}

type inmemoryCache struct {
	stash sync.Map
}

func newCache() *inmemoryCache {
	return &inmemoryCache{}
}

func (c *inmemoryCache) Store(key, value any) {
	c.stash.Store(key, value)
}

func (c *inmemoryCache) Load(key, receiver any) (bool, error) {
	if receiver == nil {
		return false, errs.ErrNilReceiver
	}

	v, ok := c.stash.Load(key)
	if !ok {
		return false, nil
	}

	if !util.CastTo(receiver, v) {
		return false, errs.ErrIncompatibleReceiver
	}

	return true, nil
}
