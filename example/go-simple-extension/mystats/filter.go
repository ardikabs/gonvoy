package mystats

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(Filter{}, gonvoy.ConfigOptions{
		MetricsPrefix: "mystats_",
	})
}

type Filter struct{}

var _ gonvoy.HttpFilter = Filter{}

func (f Filter) Name() string {
	return "mystats"
}

func (f Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	return nil
}

func (f Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"response_code", gonvoy.MustGetProperty(c, "response.code", "-"),
		"response_flags", gonvoy.MustGetProperty(c, "response.flags", "-"),
		"upstream_name", gonvoy.MustGetProperty(c, "xds.cluster_name", "-"),
		"route_name", gonvoy.MustGetProperty(c, "xds.route_name", "-"),
	).Increment(1)

	return nil
}
