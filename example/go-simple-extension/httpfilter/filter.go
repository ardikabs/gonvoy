package httpfilter

import (
	"fmt"
	"go-simple-extension/httpfilter/handler"

	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(Filter{}, gonvoy.ConfigOptions{
		BaseConfig:   new(Config),
		MetricPrefix: "gse_",

		DisabledHttpFilterPhases: []gonvoy.HttpFilterPhase{gonvoy.OnResponseBodyPhase},
		DisableStrictBodyAccess:  true,
	})
}

type Filter struct{}

var _ gonvoy.HttpFilter = &Filter{}

func (f Filter) Name() string {
	return "httpfilter"
}

func (f Filter) OnStart(c gonvoy.Context) error {
	fcfg := c.Configuration().GetFilterConfig()
	cfg, ok := fcfg.(*Config)
	if !ok {
		return fmt.Errorf("unexpected configuration type %T, expecting %T", fcfg, cfg)
	}

	c.RegisterHandler(&handler.HandlerOne{})
	c.RegisterHandler(&handler.HandlerTwo{})
	c.RegisterHandler(&handler.HandlerThree{RequestHeaders: cfg.RequestHeaders})
	return nil
}

func (f Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("httpfilter_requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
