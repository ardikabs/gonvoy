package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.HandleEcho(headersModifierFilterName,
		func(c gonvoy.EchoContext) error {
			cfg, err := gonvoy.EchoConfig[*Config](c)
			if err != nil {
				return nil
			}

			for k, v := range cfg.RequestHeaders {
				c.SetRequestHeader(k, v)
			}
			for k, v := range cfg.ResponseHeaders {
				c.SetHeader(k, v)
			}

			return c.Next()
		},
		gonvoy.WithConfig(new(Config)),
	)

	envoy.HandleEcho(echoServerFilterName,
		func(c gonvoy.EchoContext) error {
			for k, v := range c.Request().Header {
				c.Log().Info("request header --->", k, v)
			}

			return c.Next()
		},
	)
}

func main() {}

const headersModifierFilterName = "http_headers_modifier"

const echoServerFilterName = "echoserver"
