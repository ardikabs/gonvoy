package gonvoy

import (
	"errors"
	"strings"

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
}

func newInternalConfig(cc api.ConfigCallbacks, options ConfigOptions) *internalConfig {
	gc := &internalConfig{
		callbacks:       cc,
		internalCache:   newInternalCache(),
		autoReloadRoute: options.AutoReloadRoute,
		metricsPrefix:   options.MetricsPrefix,

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

func (c *internalConfig) defineCounterMetric(name string) api.CounterMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricsPrefix + name))
	return c.callbacks.DefineCounterMetric(name)
}

func (c *internalConfig) defineGaugeMetric(name string) api.GaugeMetric {
	name = strings.ToLower(util.ReplaceAllEmptySpace(c.metricsPrefix + name))
	return c.callbacks.DefineGaugeMetric(name)
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
