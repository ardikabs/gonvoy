package mystats

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

const filterName = "mystats"

func init() {
	envoy.Handle(filterName,
		gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
			return gonvoy.Handler(func(c gonvoy.Context) error {
				if err := next(c); err != nil {
					return err
				}

				host, _ := c.Attr("request.host", "-")
				method, _ := c.Attr("request.method", "-")
				code, _ := c.Attr("response.code", "-")
				flags, _ := c.Attr("response.flags", "-")
				upstream, _ := c.Attr("xds.cluster_name", "-")
				route, _ := c.Attr("xds.route_name", "-")

				c.Stats().Counter("requests_total",
					"host", host,
					"method", method,
					"response_code", code,
					"response_flags", flags,
					"upstream_name", upstream,
					"route_name", route,
				).Increment(1)

				return nil
			})
		}),
		gonvoy.MetricPrefix("mystats_"),
	)
}
