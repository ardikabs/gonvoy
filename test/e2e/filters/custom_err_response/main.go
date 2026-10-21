package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		gonvoy.Terminal{}.ServeHTTP(w, r)

		switch code := w.Status(); code {
		case http.StatusUnauthorized:
			writeMinimalJSON(w, code, "UNAUTHORIZED")
		case http.StatusTooManyRequests:
			writeMinimalJSON(w, code, "TOO_MANY_REQUESTS")
		case http.StatusServiceUnavailable:
			writeMinimalJSON(w, code, "SERVICE_UNAVAILABLE")
		}
	}))
}

func writeMinimalJSON(w gonvoy.ResponseWriter, code int, name string) {
	gonvoy.JSON(w, code, gonvoy.NewMinimalJSONResponse(name, name))
}

func main() {}

const FilterName = "custom_err_response"
