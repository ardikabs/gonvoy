package main

import (
	"fmt"

	"github.com/ardikabs/gaetway"
)

func init() {
	gonvoy.RunHttpFilter(new(Filter), gonvoy.ConfigOptions{
		FilterConfig: new(Config),
	})

	gonvoy.RunHttpFilter(new(Echoserver), gonvoy.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "http_headers_modifier"
}

func (Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	cfg, ok := c.GetFilterConfig().(*Config)
	if !ok {
		return fmt.Errorf("unexpected configuration type %T, expecting %T", c.GetFilterConfig(), cfg)
	}

	ctrl.AddHandler(&Handler{
		RequestHeaders:  cfg.RequestHeaders,
		ResponseHeaders: cfg.ResponseHeaders,
	})

	return nil
}

func (Filter) OnComplete(c gonvoy.Context) error {
	return nil
}

type Handler struct {
	gonvoy.PassthroughHttpFilterHandler

	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
}

func (h *Handler) OnRequestHeader(c gonvoy.Context) error {
	for k, v := range h.RequestHeaders {
		c.RequestHeader().Set(k, v)
	}

	return nil
}

func (h *Handler) OnResponseHeader(c gonvoy.Context) error {
	for k, v := range h.ResponseHeaders {
		c.ResponseHeader().Set(k, v)
	}

	return nil
}
