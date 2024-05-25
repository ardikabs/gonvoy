package main

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(new(Filter), gonvoy.ConfigOptions{
		MetricsPrefix: "mymetrics_",
	})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "mymetrics"
}

func (Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	ctrl.AddHandler(Handler{})
	return nil
}

func (Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"response_code", gonvoy.MustGetProperty(c, "response.code", "-"),
		"upstream_name", gonvoy.MustGetProperty(c, "xds.cluster_name", "-"),
		"route_name", gonvoy.MustGetProperty(c, "xds.route_name", "-"),
	).Increment(1)

	return nil
}

const (
	HeaderXMetricCounter = "x-metric-counter-on"
)

type Handler struct {
	gonvoy.PassthroughHttpFilterHandler
}

func (h Handler) OnRequestHeader(c gonvoy.Context) error {
	header := c.Request().Header

	if v := header.Get(HeaderXMetricCounter); v != "" {
		c.Metrics().Counter("header_appears_total", "header_value", v, "reporter", "gonvoy").Increment(1)
	}

	return nil
}
