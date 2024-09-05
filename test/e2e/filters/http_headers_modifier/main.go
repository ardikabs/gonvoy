package main

import (
	"fmt"

	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{
		FilterConfig: new(Config),
	})

	gaetway.RunHttpFilter(new(Echoserver), gaetway.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "http_headers_modifier"
}

func (Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
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

func (Filter) OnComplete(c gaetway.Context) error {
	return nil
}

type Handler struct {
	gaetway.PassthroughHttpFilterHandler

	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
}

func (h *Handler) OnRequestHeader(c gaetway.Context) error {
	for k, v := range h.RequestHeaders {
		c.RequestHeader().Set(k, v)
	}

	return nil
}

func (h *Handler) OnResponseHeader(c gaetway.Context) error {
	for k, v := range h.ResponseHeaders {
		c.ResponseHeader().Set(k, v)
	}

	return nil
}
