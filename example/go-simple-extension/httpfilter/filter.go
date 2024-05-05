package httpfilter

import (
	"go-simple-extension/httpfilter/handler"

	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilterWithConfig(&Filter{}, &Config{})
}

type Filter struct {
	Config *Config
}

var _ gonvoy.HttpFilter = &Filter{}

func (f *Filter) Name() string {
	return "httpfilter"
}

func (f *Filter) OnStart(c gonvoy.Context) {
	fConfig := c.Configuration().GetFilterConfig()
	cfg, ok := fConfig.(*Config)
	if !ok {
		// assign default config
		f.Config = &Config{}
	}

	f.Config = cfg
}

func (f *Filter) RegisterHttpFilterHandler(c gonvoy.Context, mgr gonvoy.HttpFilterHandlerManager) {
	mgr.Register(&handler.HandlerOne{})
	mgr.Register(&handler.HandlerTwo{})
	mgr.Register(&handler.HandlerThree{RequestHeaders: f.Config.RequestHeaders})
}

func (f *Filter) OnComplete(c gonvoy.Context) {
	c.Metrics().Counter("go_simple_extension",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)
}
