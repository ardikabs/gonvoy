package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(rerouteFilterName,
		func(c gonvoy.Context) error {
			if v := c.Header("x-route-to"); v == "staticreply" {
				c.SetRequestHeader("x-upstream-name", "staticreply")
			}

			if v := c.Header("x-path-changed-to"); v == "staticreply" {
				c.SetPath("/staticreply")
			}

			if v := c.Header("x-changed-host"); v == "true" {
				c.SetHost("staticreply.svc")
				c.SetMethod(http.MethodPost)
			}

			if v := c.Header("x-refresh"); v == "true" {
				c.RefreshRoute()
				return nil
			}

			if v := c.Header("x-override-to"); v != "" {
				c.OverrideUpstream(v, true)
				return nil
			}

			if c.Header("x-route-to") != "" || c.Header("x-path-changed-to") != "" || c.Header("x-changed-host") == "true" {
				c.Reroute()
			}

			return nil
		},
		gonvoy.ReloadRoute(),
	)
}

func main() {}

const rerouteFilterName = "http_reroute"
