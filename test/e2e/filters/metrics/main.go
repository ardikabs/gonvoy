package main

import (
	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{
		MetricsPrefix: "mymetrics_",
	})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "mymetrics"
}

func (Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
	ctrl.AddHandler(Handler{})
	return nil
}

func (Filter) OnComplete(c gaetway.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gaetway.MustGetProperty(c, "request.host", "-"),
		"method", gaetway.MustGetProperty(c, "request.method", "-"),
		"response_code", gaetway.MustGetProperty(c, "response.code", "-"),
		"upstream_name", gaetway.MustGetProperty(c, "xds.cluster_name", "-"),
		"route_name", gaetway.MustGetProperty(c, "xds.route_name", "-"),
	).Increment(1)

	return nil
}

const (
	HeaderXMetricCounter = "x-metric-counter-on"
)

type Handler struct {
	gaetway.PassthroughHttpFilterHandler
}

func (h Handler) OnRequestHeader(c gaetway.Context) error {
	header := c.Request().Header

	if v := header.Get(HeaderXMetricCounter); v != "" {
		c.Metrics().Counter("header_appears_total", "header_value", v, "reporter", "gaetway").Increment(1)
	}

	return nil
}
