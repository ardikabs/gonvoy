package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
)

func Two(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		log := gonvoy.Log(r).WithName("handlerTwo")

		r.Header.Add("x-user-id", "0")
		r.Header.Add("x-user-id", "1")

		header := r.Header

		if header.Get("x-error") == "403" {
			plain(w, http.StatusForbidden, "access denied")
			return
		}

		if header.Get("x-error") == "429" {
			plain(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		if header.Get("x-error") == "503" {
			plain(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}

		data := new(globaldata)
		if ok, err := gonvoy.SharedCache(r).Load(GLOBAL, &data); ok && err == nil {
			data.Time2 = time.Now()
			log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
			gonvoy.SharedCache(r).Store(GLOBAL, data)
		}

		log.Info("handling request", "host", r.Host, "path", r.URL.Path, "method", r.Method, "query", r.URL.Query())

		next.ServeHTTP(w, r)
	})
}
