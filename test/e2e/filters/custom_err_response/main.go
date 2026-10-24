package main

import (
	"encoding/json"
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.HandleEcho(FilterName, func(c gonvoy.EchoContext) error {
		if err := c.Next(); err != nil {
			return err
		}

		switch code := c.Status(); code {
		case http.StatusUnauthorized:
			return writeMinimalJSON(c, code, "UNAUTHORIZED")
		case http.StatusTooManyRequests:
			return writeMinimalJSON(c, code, "TOO_MANY_REQUESTS")
		case http.StatusServiceUnavailable:
			return writeMinimalJSON(c, code, "SERVICE_UNAVAILABLE")
		}

		return nil
	})
}

func writeMinimalJSON(c gonvoy.EchoContext, code int, name string) error {
	return c.JSON(code, json.RawMessage(gonvoy.NewMinimalJSONResponse(name, name)))
}

func main() {}

const FilterName = "custom_err_response"
