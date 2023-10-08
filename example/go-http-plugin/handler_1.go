package main

import (
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerOne struct{}

func (h *HandlerOne) RequestHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.RequestHeader().Add("x-key-id", "0")
		c.RequestHeader().Add("x-key-id", "1")

		if c.Request().Header.Get("x-error") == "401" {
			return fmt.Errorf("intentionally return unauthorized, %w", errs.ErrUnauthorized)
		}

		if c.Request().Header.Get("x-error") == "200" {
			if err := func() error {
				return c.JSON(http.StatusOK, envoy.CreateSimpleJSONBody("SUCCESS", "SUCCESS"), nil)
			}(); err != nil {
				return err
			}
		}

		c.Log(envoy.ErrorLevel, fmt.Sprintln(c.Request().Host, c.Request().URL.Path, c.Request().Method, c.Request().URL.Query()))
		return next(c)
	}
}

func (h *HandlerOne) ResponseHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.ResponseHeader().Set("via", "gateway.ardikabs.com")

		return next(c)
	}
}
