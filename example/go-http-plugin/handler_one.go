package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerOne struct{}

func (h *HandlerOne) RequestHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		log := c.Log().WithName("handlerOne").WithName("outer").WithName("inner")

		c.RequestHeader().Add("x-key-id", "0")
		c.RequestHeader().Add("x-key-id", "1")

		if c.Request().Header.Get("x-error") == "401" {
			return fmt.Errorf("intentionally return unauthorized, %w", errs.ErrUnauthorized)
		}

		if c.Request().Header.Get("x-error") == "5xx" {
			return errors.New("intentionally return unidentified error")
		}

		if c.Request().Header.Get("x-error") == "200" {
			if err := func() error {
				return c.JSON(http.StatusOK, envoy.CreateSimpleJSONBody("SUCCESS", "SUCCESS"), nil)
			}(); err != nil {
				return err
			}
		}

		if c.Request().Header.Get("x-error") == "panick" {
			panicNilMapOuter()
		}

		log.Error(errors.New("error from handler one"), "handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
		return next(c)
	}
}

func (h *HandlerOne) ResponseHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.ResponseHeader().Set("via", "gateway.ardikabs.com")
		return next(c)
	}
}

func panicNilMapOuter() {
	panicNilMapInner()
}

func panicNilMapInner() {
	var a map[string]string
	a["blbl"] = "sdasd"
}
