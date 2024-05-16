package gonvoy

import (
	"errors"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

func (c *context) GetProperty(name, defaultVal string) (string, error) {
	value, err := c.callback.GetProperty(name)
	if err != nil {
		if errors.Is(err, api.ErrValueNotFound) {
			return defaultVal, nil
		}

		return value, err
	}

	if value == "" {
		return defaultVal, nil
	}

	return value, nil
}

func (c *context) StreamInfo() api.StreamInfo {
	return c.callback.StreamInfo()
}

func (c *context) GetFilterConfig() interface{} {
	return c.filterConfig
}

func (c *context) GlobalCache() Cache {
	return c.globalCache
}

func (c *context) LocalCache() Cache {
	return c.localCache
}

func (c *context) Log() logr.Logger {
	return c.logger
}

func (c *context) Metrics() Metrics {
	return c.metrics
}

func (c *context) SetErrorHandler(e ErrorHandler) {
	c.manager.SetErrorHandler(e)
}

func (c *context) AddHTTPFilterHandler(handler HttpFilterHandler) {
	c.manager.AddHTTPFilterHandler(handler)
}
