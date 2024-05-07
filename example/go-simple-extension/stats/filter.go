package stats

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(Filter{}, gonvoy.ConfigOptions{})
}

type Filter struct{}

var _ gonvoy.HttpFilter = Filter{}

func (f Filter) Name() string {
	return "stats"
}

func (f Filter) OnStart(c gonvoy.Context) error {
	return nil
}

func (f Filter) OnComplete(c gonvoy.Context) error {
	c.Metrics().Counter("gse_requests_total",
		"host", gonvoy.MustGetProperty(c, "request.host", "-"),
		"method", gonvoy.MustGetProperty(c, "request.method", "-"),
		"status_code", gonvoy.MustGetProperty(c, "response.code", "-"),
	).Increment(1)

	return nil
}
