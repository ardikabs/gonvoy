package mystats

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(Filter{}, gonvoy.ConfigOptions{
		MetricPrefix: "mystats_",
	})
}

type Filter struct{}

var _ gonvoy.HttpFilter = Filter{}

func (f Filter) Name() string {
	return "mystats"
}

func (f Filter) OnBegin(c gonvoy.RuntimeContext, i *gonvoy.Instance) error {
	return nil
}

func (f Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
