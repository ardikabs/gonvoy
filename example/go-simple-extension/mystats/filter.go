package mystats

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

const filterName = "mystats"

func init() {
	envoy.Handle(filterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			gonvoy.Terminal{}.ServeHTTP(w, r)

			host, _ := gonvoy.Attr(r, "request.host", "-")
			method, _ := gonvoy.Attr(r, "request.method", "-")
			code, _ := gonvoy.Attr(r, "response.code", "-")
			flags, _ := gonvoy.Attr(r, "response.flags", "-")
			upstream, _ := gonvoy.Attr(r, "xds.cluster_name", "-")
			route, _ := gonvoy.Attr(r, "xds.route_name", "-")

			gonvoy.Stats(r).Counter("requests_total",
				"host", host,
				"method", method,
				"response_code", code,
				"response_flags", flags,
				"upstream_name", upstream,
				"route_name", route,
			).Increment(1)
		}),
		gonvoy.MetricPrefix("mystats_"),
	)
}
