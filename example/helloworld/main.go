package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle("hello", gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		w.Header().Set("x-hello", "world")
	}))
}

func main() {}
