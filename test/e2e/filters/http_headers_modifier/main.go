package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(headersModifierFilterName,
		func(c gonvoy.Context) error {
			cfg, err := gonvoy.ConfigOf[*Config](c)
			if err != nil {
				return nil
			}

			for k, v := range cfg.RequestHeaders {
				c.SetRequestHeader(k, v)
			}
			for k, v := range cfg.ResponseHeaders {
				c.SetHeader(k, v)
			}

			return nil
		},
		gonvoy.WithConfig(new(Config)),
	)

	envoy.Handle(echoServerFilterName,
		func(c gonvoy.Context) error {
			for k, v := range c.Request().Header {
				c.Log().Info("request header --->", k, v)
			}

			return nil
		},
	)
}

func main() {}

const headersModifierFilterName = "http_headers_modifier"

const echoServerFilterName = "echoserver"
