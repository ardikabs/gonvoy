package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
)

func One(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		log := gonvoy.Log(r).WithName("handlerOne").WithName("outer").WithName("inner")

		r.Header.Add("x-key-id", "0")
		r.Header.Add("x-key-id", "1")

		header := r.Header

		if header.Get("x-error") == "401" {
			unauthorized(w, fmt.Errorf("intentionally return unauthorized"))
			return
		}

		if header.Get("x-error") == "5xx" {
			http.Error(w, "intentionally return unidentified error", http.StatusInternalServerError)
			return
		}

		if header.Get("x-error") == "503" {
			plain(w, http.StatusServiceUnavailable, "service unavailable")
			return
		}

		if header.Get("x-error") == "200" {
			w.Header().Set("Content-Type", gonvoy.MIMEApplicationJSONCharsetUTF8)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(gonvoy.NewMinimalJSONResponse("SUCCESS", "SUCCESS"))
			return
		}

		if header.Get("x-skip-next-phase") == "true" {
			return
		}

		if header.Get("x-data") == "global" {
			data := new(globaldata)
			data.Name = "from-handler-one"

			if ok, err := gonvoy.SharedCache(r).Load(GLOBAL, &data); ok && err == nil {
				data.Time = time.Now()
				log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
				gonvoy.SharedCache(r).Store(GLOBAL, data)
			}
		}

		if header.Get("x-error") == "panick" {
			panicNilMapOuter()
		}

		log.Error(errors.New("error from handler one"), "handling request", "host", r.Host, "path", r.URL.Path, "method", r.Method, "query", r.URL.Query())

		next.ServeHTTP(w, r)

		w.Header().Set("via", "gateway.ardikabs.com")
	})
}

func unauthorized(w gonvoy.ResponseWriter, err error) {
	_ = err
	w.Header().Set("Content-Type", gonvoy.MIMEApplicationJSONCharsetUTF8)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write(gonvoy.NewMinimalJSONResponse("UNAUTHORIZED", "Unauthorized"))
}

func badRequest(w gonvoy.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", gonvoy.MIMEApplicationJSONCharsetUTF8)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write(gonvoy.NewMinimalJSONResponse("BAD_REQUEST", msg))
}

func plain(w gonvoy.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", gonvoy.MIMETextPlainCharsetUTF8)
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

func panicNilMapOuter() {
	panicNilMapInner()
}

// nolint:nilness
func panicNilMapInner() {
	var a map[string]string
	a["blbl"] = "sdasd"
}
