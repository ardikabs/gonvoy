package httpfilter

import (
	"fmt"
	"go-simple-extension/myfilter/handler"

	"github.com/ardikabs/gonvoy"
)

const filterName = "myfilter"

func init() {
	gonvoy.RunHttpFilter(
		filterName,
		func() gonvoy.HttpFilter {
			return new(Filter)
		},
		gonvoy.ConfigOptions{
			FilterConfig:            new(Config),
			MetricsPrefix:           "myfilter_",
			AutoReloadRoute:         true,
			DisableStrictBodyAccess: true,
			EnableRequestBodyWrite:  true,
			// EnableResponseBodyRead:         true,
			// DisableChunkedEncodingRequest:  true,
			// DisableChunkedEncodingResponse: true,
		},
	)

}

type Filter struct{}

var _ gonvoy.HttpFilter = &Filter{}

func (f *Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	fcfg := c.GetFilterConfig()
	cfg, ok := fcfg.(*Config)
	if !ok {
		return fmt.Errorf("unexpected configuration type %T, expecting %T", fcfg, cfg)
	}

	ctrl.AddHandler(&handler.HandlerOne{})
	ctrl.AddHandler(&handler.HandlerTwo{})
	ctrl.AddHandler(&handler.HandlerThree{RequestHeaders: cfg.RequestHeaders})
	return nil
}

func (f *Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
