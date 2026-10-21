package main

import (
	"net/http"
	"strconv"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(metricsFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			if v := r.Header.Get(HeaderXMetricCounter); v != "" {
				gonvoy.Stats(r).Counter("header_appears_total", "header_value", v, "reporter", "gonvoy").Increment(1)
			}

			gonvoy.Terminal{}.ServeHTTP(w, r)

			host, _ := gonvoy.Attr(r, "request.host", "-")
			method, _ := gonvoy.Attr(r, "request.method", "-")
			upstream, _ := gonvoy.Attr(r, "xds.cluster_name", "-")
			route, _ := gonvoy.Attr(r, "xds.route_name", "-")

			code := "-"
			if s := w.Status(); s != 0 {
				code = strconv.Itoa(s)
			}

			gonvoy.Stats(r).Counter("requests_total",
				"host", host,
				"method", method,
				"response_code", code,
				"upstream_name", upstream,
				"route_name", route,
			).Increment(1)
		}),
		gonvoy.MetricPrefix("mymetrics_"),
	)
}

func main() {}

const metricsFilterName = "mymetrics"

const HeaderXMetricCounter = "x-metric-counter-on"
