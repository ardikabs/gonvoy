package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(rerouteFilterName,
		func(c gonvoy.Context) error {
			if v := c.Request().Header.Get("x-route-to"); v == "staticreply" {
				c.Request().Header.Set("x-upstream-name", "staticreply")
			}

			if v := c.Request().Header.Get("x-path-changed-to"); v == "staticreply" {
				c.SetPath("/staticreply")
			}

			if v := c.Request().Header.Get("x-changed-host"); v == "true" {
				c.SetHost("staticreply.svc")
				c.SetMethod(http.MethodPost)
			}

			if v := c.Request().Header.Get("x-refresh"); v == "true" {
				c.RefreshRoute()
				return nil
			}

			if v := c.Request().Header.Get("x-override-to"); v != "" {
				c.OverrideUpstream(v, true)
				return nil
			}

			if c.Request().Header.Get("x-route-to") != "" || c.Request().Header.Get("x-path-changed-to") != "" || c.Request().Header.Get("x-changed-host") == "true" {
				c.Reroute()
			}

			return nil
		},
		gonvoy.ReloadRoute(),
	)
}

func main() {}

const rerouteFilterName = "http_reroute"
