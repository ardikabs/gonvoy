package main

import (
	"strconv"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(metricsFilterName,
		gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
			return gonvoy.Handler(func(c gonvoy.Context) error {
				if v := c.Request().Header.Get(HeaderXMetricCounter); v != "" {
					c.Stats().Counter("header_appears_total", "header_value", v, "reporter", "gonvoy").Increment(1)
				}

				if err := next(c); err != nil {
					return err
				}

				host, _ := c.Attr("request.host", "-")
				method, _ := c.Attr("request.method", "-")
				upstream, _ := c.Attr("xds.cluster_name", "-")
				route, _ := c.Attr("xds.route_name", "-")

				code := "-"
				if s := c.Status(); s != 0 {
					code = strconv.Itoa(s)
				}

				c.Stats().Counter("requests_total",
					"host", host,
					"method", method,
					"response_code", code,
					"upstream_name", upstream,
					"route_name", route,
				).Increment(1)

				return nil
			})
		}),
		gonvoy.MetricPrefix("mymetrics_"),
	)
}

func main() {}

const metricsFilterName = "mymetrics"

const HeaderXMetricCounter = "x-metric-counter-on"
