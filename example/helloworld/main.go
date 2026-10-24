package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.HandleEcho("hello", func(c gonvoy.EchoContext) error {
		c.SetHeader("x-hello", "world")
		return nil
	})
}

func main() {}
