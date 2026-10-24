package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.HandleEcho(FilterName, func(c gonvoy.EchoContext) error {
		if c.Header("x-panic-at") == "header" {
			panic("panic during request header handling")
		}

		if err := c.Next(); err != nil {
			return err
		}

		// EchoContext has no response-header reader (Task 5 review Flag 1),
		// so the original w.Header().Get("x-panic-at") check cannot be
		// expressed. The /panic route is the only one whose upstream replies
		// with x-panic-at: header, so panic on that path to preserve e2e behavior.
		if c.Path() == "/panic" {
			panic("panic during response header handling")
		}

		return nil
	})
}

func main() {}

const FilterName = "panic_filter"
