package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
)

type HandlerTwo struct {
	gonvoy.PassthroughHttpFilterHandler
}

func (h *HandlerTwo) OnRequestHeader(c gonvoy.Context) error {
	log := c.Log().WithName("handlerTwo")

	c.RequestHeader().Add("x-user-id", "0")
	c.RequestHeader().Add("x-user-id", "1")

	header := c.Request().Header

	if header.Get("x-error") == "403" {
		return c.String(http.StatusForbidden, "access denied", gonvoy.LocalReplyWithHTTPHeaders(gonvoy.NewGatewayHeaders()))
	}

	if header.Get("x-error") == "429" {
		return c.String(http.StatusTooManyRequests, "rate limit exceeded", gonvoy.LocalReplyWithHTTPHeaders(gonvoy.NewGatewayHeaders()))
	}

	if header.Get("x-error") == "503" {
		return c.String(http.StatusServiceUnavailable, "service unavailable", gonvoy.LocalReplyWithHTTPHeaders(gonvoy.NewGatewayHeaders()))
	}

	data := new(globaldata)
	if ok, err := c.GetCache().Load(GLOBAL, &data); ok && err == nil {
		data.Time2 = time.Now()
		log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
		c.GetCache().Store(GLOBAL, data)
	}

	log.Info("handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
	return nil
}

func (h *HandlerTwo) OnResponseHeader(c gonvoy.Context) error {
	return nil
}
