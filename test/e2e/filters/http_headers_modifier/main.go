package main

import (
	"fmt"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.RegisterHttpFilter(
		headersModifierFilterName,
		func() gonvoy.HttpFilter {
			return new(Filter)
		},
		gonvoy.ConfigOptions{
			FilterConfig: new(Config),
		},
	)

	envoy.RegisterHttpFilter(
		echoServerFilterName,
		func() gonvoy.HttpFilter {
			return new(Echoserver)
		},
		gonvoy.ConfigOptions{},
	)
}

func main() {}

const headersModifierFilterName = "http_headers_modifier"

type Filter struct{}

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
