package mystats

import (
	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(Filter{}, gaetway.ConfigOptions{
		MetricsPrefix: "mystats_",
	})
}

type Filter struct{}

var _ gaetway.HttpFilter = Filter{}

func (f Filter) Name() string {
	return "mystats"
}

func (f Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
	return nil
}

func (f Filter) OnComplete(c gaetway.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gaetway.MustGetProperty(c, "request.host", "-"),
		"method", gaetway.MustGetProperty(c, "request.method", "-"),
		"response_code", gaetway.MustGetProperty(c, "response.code", "-"),
		"response_flags", gaetway.MustGetProperty(c, "response.flags", "-"),
		"upstream_name", gaetway.MustGetProperty(c, "xds.cluster_name", "-"),
		"route_name", gaetway.MustGetProperty(c, "xds.route_name", "-"),
	).Increment(1)

	return nil
}
