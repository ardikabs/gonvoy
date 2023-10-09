package main

import (
	"net/http"

	"github.com/ardikabs/go-envoy"
)

type HandlerTwo struct{}

func (h *HandlerTwo) RequestHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.RequestHeader().Add("x-user-id", "0")
		c.RequestHeader().Add("x-user-id", "1")

		if c.Request().Header.Get("x-error") == "403" {
			return c.String(http.StatusForbidden, "access denied")
		}

		if c.Request().Header.Get("x-error") == "429" {
			return c.String(http.StatusTooManyRequests, "rate limit exceeded")
		}

		if c.Request().Header.Get("x-error") == "503" {
			return c.String(http.StatusServiceUnavailable, "service unavailable")
		}

		return next(c)
	}
}

func (h *HandlerTwo) ResponseHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		return next(c)
	}
}
