package httpfilter

import (
	"go-simple-extension/httpfilter/handler"

	"github.com/ardikabs/go-envoy"
)

func init() {
	envoy.RunHttpFilterWithConfig(&Filter{}, &Config{})
}

type Filter struct {
	Config *Config
}

var _ envoy.HttpFilter = &Filter{}

func (f *Filter) Name() string {
	return "httpfilter"
}

func (f *Filter) OnStart(c envoy.Context) {
	fConfig := c.Configuration().GetFilterConfig()
	cfg, ok := fConfig.(*Config)
	if !ok {
		// assign default config
		f.Config = &Config{}
	}

	f.Config = cfg
}

func (f *Filter) RegisterHttpFilterHandler(c envoy.Context, mgr envoy.HandlerManager) {
	mgr.Use(&handler.HandlerOne{})
	mgr.Use(&handler.HandlerTwo{})
	mgr.Use(&handler.HandlerThree{RequestHeaders: f.Config.RequestHeaders})
}

func (f *Filter) OnComplete(c envoy.Context) {
	c.Metrics().Counter("go_http_plugin",
		"host", envoy.MustGetProperty(c, "request.host", "-"),
		"method", envoy.MustGetProperty(c, "request.method", "-"),
		"status_code", envoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)
}
