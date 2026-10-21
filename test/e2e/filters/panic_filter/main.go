package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-panic-at") == "header" {
			panic("panic during request header handling")
		}

		gonvoy.Terminal{}.ServeHTTP(w, r)

		if w.Header().Get("x-panic-at") == "header" {
			panic("panic during response header handling")
		}
	}))
}

func main() {}

const FilterName = "panic_filter"
