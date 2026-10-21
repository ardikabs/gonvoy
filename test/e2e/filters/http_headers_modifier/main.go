package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(headersModifierFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			cfg, err := gonvoy.Config[*Config](r)
			if err != nil {
				return
			}

			for k, v := range cfg.RequestHeaders {
				r.Header.Set(k, v)
			}
			for k, v := range cfg.ResponseHeaders {
				w.Header().Set(k, v)
			}

			gonvoy.Terminal{}.ServeHTTP(w, r)
		}),
		gonvoy.WithConfig(new(Config)),
	)

	envoy.Handle(echoServerFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			for k, v := range r.Header {
				gonvoy.Log(r).Info("request header --->", k, v)
			}

			gonvoy.Terminal{}.ServeHTTP(w, r)
		}),
	)
}

func main() {}

const headersModifierFilterName = "http_headers_modifier"

const echoServerFilterName = "echoserver"
