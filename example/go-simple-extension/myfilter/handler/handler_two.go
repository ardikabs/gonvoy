package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
)

// Two handles the middle layer of the example chain.
func Two(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.Handler(func(c gonvoy.Context) error {
		log := c.Log().WithName("handlerTwo")

		r := c.Request()
		r.Header.Add("x-user-id", "0")
		r.Header.Add("x-user-id", "1")

		if c.Request().Header.Get("x-error") == "403" {
			return plain(c, http.StatusForbidden, "access denied")
		}

		if c.Request().Header.Get("x-error") == "429" {
			return plain(c, http.StatusTooManyRequests, "rate limit exceeded")
		}

		if c.Request().Header.Get("x-error") == "503" {
			return plain(c, http.StatusServiceUnavailable, "service unavailable")
		}

		data := new(globaldata)
		if ok, err := c.Cache().Load(GLOBAL, &data); ok && err == nil {
			data.Time2 = time.Now()
			log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
			c.Cache().Store(GLOBAL, data)
		}

		log.Info("handling request", "host", r.Host, "path", r.URL.Path, "method", r.Method, "query", r.URL.Query())

		return next(c)
	})
}
