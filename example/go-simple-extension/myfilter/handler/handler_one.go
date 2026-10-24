package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
)

// One handles the outer layer of the example chain.
func One(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.Handler(func(c gonvoy.Context) error {
		log := c.Log().WithName("handlerOne").WithName("outer").WithName("inner")

		r := c.Request()
		r.Header.Add("x-key-id", "0")
		r.Header.Add("x-key-id", "1")

		if c.Header("x-error") == "401" {
			return unauthorized(c, fmt.Errorf("intentionally return unauthorized"))
		}

		if c.Header("x-error") == "5xx" {
			return c.String(http.StatusInternalServerError, "intentionally return unidentified error")
		}

		if c.Header("x-error") == "503" {
			return plain(c, http.StatusServiceUnavailable, "service unavailable")
		}

		if c.Header("x-error") == "200" {
			return c.JSON(http.StatusOK, json.RawMessage(gonvoy.NewMinimalJSONResponse("SUCCESS", "SUCCESS")))
		}

		if c.Header("x-skip-next-phase") == "true" {
			return nil
		}

		if c.Header("x-data") == "global" {
			data := new(globaldata)
			data.Name = "from-handler-one"

			if ok, err := c.Cache().Load(GLOBAL, &data); ok && err == nil {
				data.Time = time.Now()
				log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
				c.Cache().Store(GLOBAL, data)
			}
		}

		if c.Header("x-error") == "panick" {
			panicNilMapOuter()
		}

		log.Error(errors.New("error from handler one"), "handling request", "host", r.Host, "path", r.URL.Path, "method", r.Method, "query", r.URL.Query())

		if err := next(c); err != nil {
			return err
		}

		c.SetHeader("via", "gateway.ardikabs.com")

		return nil
	})
}

func unauthorized(c gonvoy.Context, err error) error {
	_ = err
	return c.JSON(http.StatusUnauthorized, json.RawMessage(gonvoy.NewMinimalJSONResponse("UNAUTHORIZED", "Unauthorized")))
}

func badRequest(c gonvoy.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, json.RawMessage(gonvoy.NewMinimalJSONResponse("BAD_REQUEST", msg)))
}

func plain(c gonvoy.Context, code int, body string) error {
	return c.String(code, body)
}

func panicNilMapOuter() {
	panicNilMapInner()
}

// nolint:nilness
func panicNilMapInner() {
	var a map[string]string
	a["blbl"] = "sdasd"
}
