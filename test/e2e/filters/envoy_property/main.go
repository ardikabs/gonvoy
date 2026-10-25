package main

import (
	"strconv"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName,
		gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
			return gonvoy.Handler(func(c gonvoy.Context) error {
				host, _ := c.Attr("request.host", "-")
				method, _ := c.Attr("request.method", "-")
				path, _ := c.Attr("request.path", "-")

				c.Log().Info("envoy property request attrs", "host", host, "method", method, "path", path)

				if err := next(c); err != nil {
					return err
				}

				cluster, _ := c.Attr("xds.cluster_name", "-")
				route, _ := c.Attr("xds.route_name", "-")

				code, _ := c.Attr("response.code", "-")
				if code == "-" && c.Status() != 0 {
					code = strconv.Itoa(c.Status())
				}

				missing, missingOK := c.Attr("nonexistent.key", "fallback")

				c.SetHeader("x-request-host", host)
				c.SetHeader("x-request-method", method)
				c.SetHeader("x-request-path", path)
				c.SetHeader("x-xds-cluster", cluster)
				c.SetHeader("x-xds-route", route)
				c.SetHeader("x-response-code", code)
				c.SetHeader("x-missing-value", missing)
				c.SetHeader("x-missing-ok", strconv.FormatBool(missingOK))

				c.Log().Info("envoy property response attrs", "cluster", cluster, "route", route, "code", code, "missing", missing, "missing_ok", missingOK)

				return nil
			})
		}),
	)
}

func main() {}

const FilterName = "envoy_property"
