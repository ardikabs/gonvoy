package httpfilter

import (
	"go-simple-extension/myfilter/handler"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

const filterName = "myfilter"

func init() {
	envoy.Handle(filterName,
		gonvoy.Use(
			CountRequests,
			handler.One,
			handler.Two,
			handler.Three,
		),
		gonvoy.WithConfig(new(handler.Config)),
		gonvoy.MetricPrefix("myfilter_"),
		gonvoy.ReloadRoute(),
		gonvoy.OpenBodyAccess(),
		gonvoy.ReadRequestBody(),
		gonvoy.WriteRequestBody(),
	)
}

// CountRequests records a per-request counter after the chain runs,
// mirroring the old OnComplete hook.
func CountRequests(gonvoy.Handler) gonvoy.Handler {
	return gonvoy.Echo(func(c gonvoy.EchoContext) error {
		if err := c.Next(); err != nil {
			return err
		}

		host, _ := c.Attr("request.host", "-")
		method, _ := c.Attr("request.method", "-")

		c.Stats().Counter("requests_total",
			"host", host,
			"method", method,
			"status_code", "-",
		).Increment(1)

		return nil
	})
}
