package gonvoy

import (
	"strings"
	"sync"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type globalConfig struct {
	filterConfig  interface{}
	callbacks     api.ConfigCallbacks
	internalCache Cache
	metricPrefix  string

	strictBodyAccess       bool
	allowRequestBodyRead   bool
	allowRequestBodyWrite  bool
	allowResponseBodyRead  bool
	allowResponseBodyWrite bool
}

func newGlobalConfig(options ConfigOptions) *globalConfig {
	gc := &globalConfig{
		internalCache: newCache(),
		metricPrefix:  options.MetricPrefix,

		strictBodyAccess:       !options.DisableStrictBodyAccess,
		allowRequestBodyRead:   options.EnableRequestBodyRead,
		allowRequestBodyWrite:  options.EnableRequestBodyWrite,
		allowResponseBodyRead:  options.EnableResponseBodyRead,
		allowResponseBodyWrite: options.EnableResponseBodyWrite,
	}

	return gc

}

func (c *globalConfig) defineCounterMetric(name string) api.CounterMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricPrefix + name))
	return c.callbacks.DefineCounterMetric(name)
}

func (c *globalConfig) defineGaugeMetric(name string) api.GaugeMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricPrefix + name))
	return c.callbacks.DefineGaugeMetric(name)
}

func (c *globalConfig) defineHistogramMetric(name string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

// Cache is an interface that defines methods for storing and retrieving data in an internal cache.
// It is designed to maintain data persistently throughout Envoy's lifespan.
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
