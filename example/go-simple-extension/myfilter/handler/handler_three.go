package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ardikabs/gonvoy"
)

func Three(next gonvoy.Handler) gonvoy.Handler {
	return gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
		log := gonvoy.Log(r).WithName("handlerThree")

		var reqHeaders map[string]string
		if cfg, err := gonvoy.Config[*Config](r); err == nil && cfg != nil {
			reqHeaders = cfg.RequestHeaders
		}

		for k, v := range reqHeaders {
			r.Header.Set(k, v)
		}

		data := new(globaldata)
		if ok, err := gonvoy.SharedCache(r).Load(GLOBAL, &data); ok && err == nil {
			data.Time3 = time.Now()
			log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
			gonvoy.SharedCache(r).Store(GLOBAL, data)
		}

		log.Info("handling request", "request", r.Header)

		if ct := r.Header.Get(gonvoy.HeaderContentType); strings.Contains(ct, gonvoy.MIMEApplicationJSON) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			reqBody := make(map[string]interface{})
			if err := json.Unmarshal(body, &reqBody); err != nil {
				badRequest(w, "unexpected payload")
				return
			}

			reqBody["newData"] = "newValue"
			reqBody["handlerName"] = "HandlerThree"
			reqBody["phase"] = "HTTPRequest"

			replaced, err := json.Marshal(reqBody)
			if err != nil {
				badRequest(w, "unexpected payload")
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(replaced))
		} else {
			log.Info("payload size", "size", len(mustReadAll(r)))
		}

		next.ServeHTTP(w, r)

		if ct := w.Header().Get(gonvoy.HeaderContentType); !strings.Contains(ct, gonvoy.MIMEApplicationJSON) {
			return
		}

		resp, err := gonvoy.ResponseBody(r)
		if err != nil {
			return
		}

		raw, err := io.ReadAll(resp)
		if err != nil {
			return
		}

		respBody := make(map[string]interface{})
		if err := json.Unmarshal(raw, &respBody); err != nil {
			gonvoy.Log(r).Error(err, fmt.Sprintf("expecting data type %T, got %v", respBody, string(raw)))
			gonvoy.Log(r).Info("skipping response body manipulation ...")
			return
		}

		respBody["newData"] = "newValue"
		respBody["handlerName"] = "HandlerThree"
		respBody["phase"] = "HTTPResponse"

		changed, err := json.Marshal(respBody)
		if err != nil {
			return
		}

		_, _ = w.Write(changed)
	})
}

func mustReadAll(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	return b
}
