package main

import (
	"net/http"
	"strconv"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, func(c gonvoy.Context) error {
		if c.Path() != "/search" {
			return nil
		}

		q := c.QueryParam("q")
		if q == "" {
			return c.String(http.StatusBadRequest, "missing required query param q")
		}

		if mode := c.QueryParam("mode"); mode != "" && mode != "READ" && mode != "WRITE" {
			return c.String(http.StatusBadRequest, "invalid mode, must be READ or WRITE")
		}

		if limitRaw := c.QueryParam("limit"); limitRaw != "" {
			limit, err := strconv.Atoi(limitRaw)
			if err != nil || limit < 1 || limit > 100 {
				return c.String(http.StatusBadRequest, "invalid limit, must be int 1..100")
			}
		}

		c.Log().Info("query validated q=" + q + " path=" + c.Path() + " tag=" + c.QueryParam("tag"))

		return nil
	})
}

func main() {}

const FilterName = "http_query_validation"
