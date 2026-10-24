package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
		return gonvoy.Handler(func(c gonvoy.Context) error {
			if c.Header("x-panic-at") == "header" {
				panic("panic during request header handling")
			}

			if err := next(c); err != nil {
				return err
			}

			// Context has no response-header reader, so the original
			// w.Header().Get("x-panic-at") check cannot be expressed. The
			// /panic route is the only one whose upstream replies with
			// x-panic-at: header, so panic on that path to preserve e2e behavior.
			if c.Path() == "/panic" {
				panic("panic during response header handling")
			}

			return nil
		})
	}))
}

func main() {}

const FilterName = "panic_filter"
