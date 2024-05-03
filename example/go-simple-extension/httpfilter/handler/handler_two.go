package handler

import (
	"net/http"

	"github.com/ardikabs/go-envoy"
)

type HandlerTwo struct {
	envoy.PassthroughHttpFilterHandler
}

func (h *HandlerTwo) OnRequestHeader(c envoy.Context, header http.Header) error {
	log := c.Log().WithName("handlerTwo")

	c.RequestHeader().Add("x-user-id", "0")
	c.RequestHeader().Add("x-user-id", "1")

	if c.Request().Header.Get("x-error") == "403" {
		return c.String(http.StatusForbidden, "access denied", envoy.NewGatewayHeaders())
	}

	if c.Request().Header.Get("x-error") == "429" {
		return c.String(http.StatusTooManyRequests, "rate limit exceeded", envoy.NewGatewayHeaders())
	}

	if c.Request().Header.Get("x-error") == "503" {
		return c.String(http.StatusServiceUnavailable, "service unavailable", envoy.NewGatewayHeaders())
	}

	log.Info("handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
	return nil
}

func (h *HandlerTwo) OnResponseHeader(c envoy.Context, header http.Header) error {
	return nil
}
