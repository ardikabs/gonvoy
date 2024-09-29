package gonvoy

import (
	"errors"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

func (c *context) StreamInfo() api.StreamInfo {
	return c.cb.StreamInfo()
}

func (c *context) GetProperty(name, defaultVal string) (string, error) {
	value, err := c.cb.GetProperty(name)
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

func (c *context) GetFilterConfig() interface{} {
	return c.filterConfig
}

func (c *context) GetCache() Cache {
	return c.cache
}

func (c *context) Log() logr.Logger {
	return c.logger
}

func (c *context) Metrics() Metrics {
	return c.metrics
}
