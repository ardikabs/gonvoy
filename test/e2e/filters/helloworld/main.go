package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		gonvoy.Log(r).Info("Hello World from the helloworld HTTP filter")

		if r.Header.Get("x-drain") == "true" {
			gonvoy.DrainConnection(r)
		}
	}))
}

func main() {}

const FilterName = "helloworld"
