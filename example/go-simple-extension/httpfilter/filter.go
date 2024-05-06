package httpfilter

import (
	"fmt"
	"go-simple-extension/httpfilter/handler"

	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilterWithConfig(&Filter{}, &Config{})
}

type Filter struct{}

var _ gonvoy.HttpFilter = &Filter{}

func (f *Filter) Name() string {
	return "httpfilter"
}

func (f *Filter) OnStart(c gonvoy.Context) error {
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

func (f *Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("go_simple_extension",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
