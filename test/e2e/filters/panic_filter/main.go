package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
		return gonvoy.Handler(func(c gonvoy.Context) error {
			if c.Request().Header.Get("x-panic-at") == "header" {
				panic("panic during request header handling")
			}

			if err := next(c); err != nil {
				return err
			}

			if c.Response().Header.Get("x-panic-at") != "" {
				panic("panic during response header handling")
			}

			return nil
		})
	}))
}

func main() {}

const FilterName = "panic_filter"
