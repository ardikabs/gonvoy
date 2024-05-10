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

func (h *HandlerTwo) OnRequestHeader(c gonvoy.Context, header http.Header) error {
	log := c.Log().WithName("handlerTwo")

	c.RequestHeader().Add("x-user-id", "0")
	c.RequestHeader().Add("x-user-id", "1")

	if c.Request().Header.Get("x-error") == "403" {
		return c.String(http.StatusForbidden, "access denied", gonvoy.NewGatewayHeaders())
	}

	if c.Request().Header.Get("x-error") == "429" {
		return c.String(http.StatusTooManyRequests, "rate limit exceeded", gonvoy.NewGatewayHeaders())
	}

	if c.Request().Header.Get("x-error") == "503" {
		return c.String(http.StatusServiceUnavailable, "service unavailable", gonvoy.NewGatewayHeaders())
	}

	localdata := localdata{}
	if ok, err := c.LocalCache().Load(LocalKey, &localdata); ok && err == nil {
		if localdata.Foo != nil {
			localdata.Foo.Name = "from-handler-two"
		}

		c.LocalCache().Store(LocalKey, localdata)
		log.Info("localdata looks good", "data", localdata, "pointer", fmt.Sprintf("%p", localdata.Foo))
	}

	data := new(globaldata)
	if ok, err := c.GlobalCache().Load(GLOBAL, &data); ok && err == nil {
		data.Time2 = time.Now()
		log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
		c.GlobalCache().Store(GLOBAL, data)
	}

	log.Info("handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
	return nil
}

func (h *HandlerTwo) OnResponseHeader(c gonvoy.Context, header http.Header) error {
	return nil
}
