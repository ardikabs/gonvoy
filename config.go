package gonvoy

import (
	"errors"
	"strings"
	"sync"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	errInternalConfigNotFound = errors.New("internal config not found")
)

type internalConfig struct {
	filterConfig  interface{}
	callbacks     api.ConfigCallbacks
	internalCache Cache
	metricsPrefix string

	strictBodyAccess                bool
	allowRequestBodyRead            bool
	allowRequestBodyWrite           bool
	allowResponseBodyRead           bool
	allowResponseBodyWrite          bool
	preserveContentLengthOnRequest  bool
	preserveContentLengthOnResponse bool

	autoReloadRoute bool

	// metricCache holds shared metric handles defined once via cgo and reused
	// across streams. Pointer so config copies (parent/child merge) share it
	// without copying a mutex.
	metricCache *metricHandleCache
}

// metricHandleCache memoizes DefineCounter/GaugeMetric handles.
type metricHandleCache struct {
	mu       sync.Mutex
	counters map[string]api.CounterMetric
	gauges   map[string]api.GaugeMetric
}

func newInternalConfig(options ConfigOptions) *internalConfig {
	gc := &internalConfig{
		internalCache:   newInternalCache(),
		autoReloadRoute: options.AutoReloadRoute,
		metricsPrefix:   options.MetricsPrefix,
		metricCache:     &metricHandleCache{},

		strictBodyAccess:                !options.DisableStrictBodyAccess,
		allowRequestBodyRead:            options.EnableRequestBodyRead,
		allowRequestBodyWrite:           options.EnableRequestBodyWrite,
		allowResponseBodyRead:           options.EnableResponseBodyRead,
		allowResponseBodyWrite:          options.EnableResponseBodyWrite,
		preserveContentLengthOnRequest:  options.DisableChunkedEncodingRequest,
		preserveContentLengthOnResponse: options.DisableChunkedEncodingResponse,
	}

	return gc

}

// Destroy releases resources held by the filter config. It forwards to the
// user config when it implements Destroy. The framework may call Destroy
// once per config instance it created (parent and merged route configs
// share nothing), so a user Destroy must be idempotent.
func (c *internalConfig) Destroy() {
	if d, ok := c.filterConfig.(interface{ Destroy() }); ok {
		d.Destroy()
	}
}

func (c *internalConfig) defineCounterMetric(name string) api.CounterMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricsPrefix + name))

	mc := c.metricCache
	if mc == nil {
		return c.callbacks.DefineCounterMetric(name)
	}

	mc.mu.Lock()
	if mc.counters == nil {
		mc.counters = make(map[string]api.CounterMetric)
	}
	if m, ok := mc.counters[name]; ok {
		mc.mu.Unlock()
		return m
	}
	mc.mu.Unlock()

	m := c.callbacks.DefineCounterMetric(name)

	mc.mu.Lock()
	if existing, ok := mc.counters[name]; ok {
		mc.mu.Unlock()
		return existing
	}
	mc.counters[name] = m
	mc.mu.Unlock()

	return m
}

func (c *internalConfig) defineGaugeMetric(name string) api.GaugeMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricsPrefix + name))

	mc := c.metricCache
	if mc == nil {
		return c.callbacks.DefineGaugeMetric(name)
	}

	mc.mu.Lock()
	if mc.gauges == nil {
		mc.gauges = make(map[string]api.GaugeMetric)
	}
	if m, ok := mc.gauges[name]; ok {
		mc.mu.Unlock()
		return m
	}
	mc.mu.Unlock()

	m := c.callbacks.DefineGaugeMetric(name)

	mc.mu.Lock()
	if existing, ok := mc.gauges[name]; ok {
		mc.mu.Unlock()
		return existing
	}
	mc.gauges[name] = m
	mc.mu.Unlock()

	return m
}

func (c *internalConfig) defineHistogramMetric(name string) api.HistogramMetric {
	panic("NOT IMPLEMENTED")
}

func validateFilterConfig(filterConfig interface{}) error {
	type validator interface {
		Validate() error
	}

	if validate, ok := filterConfig.(validator); ok {
		return validate.Validate()
	}

	return nil
}

func applyInternalConfig(c *context, cfg *internalConfig) error {
	if cfg == nil {
		return errInternalConfigNotFound
	}

	if err := validateFilterConfig(cfg.filterConfig); err != nil {
		return err
	}

	c.filterConfig = cfg.filterConfig
	c.cache = cfg.internalCache
	c.metrics = newMetrics(cfg.defineCounterMetric, cfg.defineGaugeMetric, cfg.defineHistogramMetric)

	c.autoReloadRoute = cfg.autoReloadRoute

	c.strictBodyAccess = cfg.strictBodyAccess
	c.requestBodyAccessRead = cfg.allowRequestBodyRead
	c.requestBodyAccessWrite = cfg.allowRequestBodyWrite
	c.responseBodyAccessRead = cfg.allowResponseBodyRead
	c.responseBodyAccessWrite = cfg.allowResponseBodyWrite
	c.preserveContentLengthOnRequest = cfg.preserveContentLengthOnRequest
	c.preserveContentLengthOnResponse = cfg.preserveContentLengthOnResponse
	return nil
}
