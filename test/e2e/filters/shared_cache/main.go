package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(FilterName, func(c gonvoy.Context) error {
		switch c.Path() {
		case "/store":
			key := c.QueryParam("key")
			value := c.QueryParam("value")
			if key == "" || value == "" {
				return c.String(http.StatusBadRequest, "key and value query params are required")
			}
			c.Cache().Store(key, value)
			c.Log().Info("shared cache store", "key", key, "value", value)
			return c.String(http.StatusOK, value)
		case "/load":
			key := c.QueryParam("key")
			if key == "" {
				return c.String(http.StatusBadRequest, "key query param is required")
			}
			var v string
			ok, err := c.Cache().Load(key, &v)
			if err != nil {
				c.Log().Info("shared cache load error", "key", key, "error", err.Error())
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if !ok {
				return c.String(http.StatusNotFound, "not found")
			}
			c.Log().Info("shared cache hit", "key", key, "value", v)
			return c.String(http.StatusOK, v)
		case "/store-struct":
			key := c.QueryParam("key")
			if key == "" {
				return c.String(http.StatusBadRequest, "key query param is required")
			}
			c.Cache().Store(key, cachedData{Value: "struct-value"})
			c.Log().Info("shared cache store struct", "key", key)
			return c.String(http.StatusOK, "stored")
		case "/load-struct-bad":
			key := c.QueryParam("key")
			if key == "" {
				return c.String(http.StatusBadRequest, "key query param is required")
			}
			var v string
			ok, err := c.Cache().Load(key, &v)
			if err != nil {
				c.Log().Info("shared cache load type mismatch", "key", key, "error", err.Error())
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if !ok {
				return c.String(http.StatusNotFound, "not found")
			}
			return c.String(http.StatusOK, v)
		default:
			return nil
		}
	})
}

func main() {}

type cachedData struct {
	Value string
}

// FilterName is the Envoy filter name for the shared cache fixture.
const FilterName = "shared_cache"
