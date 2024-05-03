package httpfilter

import (
	"go-simple-extension/httpfilter/handler"

	"github.com/ardikabs/go-envoy"
)

type Config struct {
	RequestHeaders map[string]string `json:"request_headers,omitempty" envoy:"mergeable,preserve"`
}

type Filter struct {
	Config *Config
}

var _ envoy.HttpFilter = &Filter{}

func (f *Filter) Name() string {
	return "httpfilter"
}

func (f *Filter) OnStart(c envoy.Context) {}

func (f *Filter) Handlers(c envoy.Context) []envoy.HttpFilterHandler {
	fConfig := c.Configuration().GetFilterConfig()
	config, ok := fConfig.(*Config)
	if ok {
		f.Config = config
	}

	return []envoy.HttpFilterHandler{
		&handler.HandlerOne{},
		&handler.HandlerTwo{},
		&handler.HandlerThree{RequestHeaders: f.Config.RequestHeaders},
	}
}

func (f *Filter) OnComplete(c envoy.Context) {
	c.Metrics().Counter("go_http_plugin",
		"host", envoy.MustGetProperty(c, "request.host", "-"),
		"method", envoy.MustGetProperty(c, "request.method", "-"),
		"status_code", envoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)
}
