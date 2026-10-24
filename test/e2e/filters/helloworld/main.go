package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.HandleEcho(FilterName, func(c gonvoy.EchoContext) error {
		c.Log().Info("Hello World from the helloworld HTTP filter")

		if c.Header("x-drain") == "true" {
			c.Drain()
		}

		return nil
	})
}

func main() {}

const FilterName = "helloworld"
