package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(rerouteFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			header := r.Header

			if v := header.Get("x-route-to"); v == "staticreply" {
				r.Header.Set("x-upstream-name", "staticreply")
			}

			if v := header.Get("x-path-changed-to"); v == "staticreply" {
				r.URL.Path = "/staticreply"
			}

			if v := header.Get("x-changed-host"); v == "true" {
				r.Host = "staticreply.svc"
				r.Method = http.MethodPost
			}

			if r.Header.Get("x-route-to") != "" || r.Header.Get("x-path-changed-to") != "" || r.Header.Get("x-changed-host") == "true" {
				gonvoy.Reroute(r)
			}

			gonvoy.Terminal{}.ServeHTTP(w, r)
		}),
		gonvoy.ReloadRoute(),
	)
}

func main() {}

const rerouteFilterName = "http_reroute"
