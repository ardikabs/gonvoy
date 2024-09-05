package httpfilter

import (
	"fmt"
	"go-simple-extension/myfilter/handler"

	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{
		FilterConfig:            new(Config),
		MetricsPrefix:           "myfilter_",
		AutoReloadRoute:         true,
		DisableStrictBodyAccess: true,
		EnableRequestBodyWrite:  true,
		// EnableResponseBodyRead:         true,
		// DisableChunkedEncodingRequest:  true,
		// DisableChunkedEncodingResponse: true,
	})

}

type Filter struct{}

var _ gaetway.HttpFilter = &Filter{}

func (f *Filter) Name() string {
	return "myfilter"
}

func (f *Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
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

func (f *Filter) OnComplete(c gaetway.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gaetway.MustGetProperty(c, "request.host", "-"),
		"method", gaetway.MustGetProperty(c, "request.method", "-"),
		"status_code", gaetway.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
