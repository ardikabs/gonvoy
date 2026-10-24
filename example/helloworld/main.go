package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle("hello", gonvoy.Handler(func(c gonvoy.Context) error {
		c.SetHeader("x-hello", "world")
		return nil
	}))
}

func main() {}
