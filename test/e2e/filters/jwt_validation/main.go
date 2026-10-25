package main

import (
	"encoding/json"
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, func(c gonvoy.Context) error {
		claim := c.Request().Header.Get("x-claim-name")
		if claim == "" {
			return c.JSON(http.StatusUnauthorized, json.RawMessage(gonvoy.NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED")))
		}

		c.Response().Header.Set("x-claim-name", claim)
		c.Log().Info("JWT validated from x-claim-name header")

		return nil
	})
}

func main() {}

const FilterName = "jwt_validation"
