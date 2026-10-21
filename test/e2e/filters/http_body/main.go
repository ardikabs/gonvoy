package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.Handle(bodyReadFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			cfg, err := gonvoy.Config[*Config](r)
			if err != nil || !cfg.EnableRead {
				return
			}

			if v := r.Header.Get("x-inspect-body"); v != "true" {
				gonvoy.Terminal{}.ServeHTTP(w, r)
				return
			}

			tryWriteOnRequest := r.Header.Get("x-try-write") == "request"
			tryWriteOnResponse := r.Header.Get("x-try-write") == "response"

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			gonvoy.Log(r).Info("request body payload --->", "data", string(body), "size", len(body), "mode", "READ")

			if tryWriteOnRequest {
				newPayload := map[string]interface{}{"data": string(body)}

				replaced, err := json.Marshal(newPayload)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				r.Body = io.NopCloser(bytes.NewReader(replaced))
			}

			gonvoy.Terminal{}.ServeHTTP(w, r)

			resp, err := gonvoy.ResponseBody(r)
			if err != nil {
				return
			}

			raw, err := io.ReadAll(resp)
			if err != nil {
				return
			}

			payload := make(map[string]interface{})
			if err := json.Unmarshal(raw, &payload); err != nil {
				http.Error(w, fmt.Sprintf("response payload failed during unmarshal: %v", err), http.StatusInternalServerError)
				return
			}

			gonvoy.Log(r).Info("response body payload --->", "state", payload["state"], "mode", "READ")

			if tryWriteOnResponse {
				payload["data"] = string(raw)

				changed, err := json.Marshal(payload)
				if err != nil {
					return
				}

				_, _ = w.Write(changed)
			}
		}),
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.ReadRequestBody(),
		gonvoy.ReadResponseBody(),
	)

	envoy.Handle(bodyWriteFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			cfg, err := gonvoy.Config[*Config](r)
			if err != nil || !cfg.EnableWrite {
				return
			}

			if v := r.Header.Get("x-modify-body"); v != "true" {
				gonvoy.Terminal{}.ServeHTTP(w, r)
				return
			}

			signature := r.Header.Get("x-signature")
			if signature == "" {
				http.Error(w, "signature can not be empty", http.StatusBadRequest)
				return
			}

			raw, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Mirror the old phase handler: request replacement only happens
			// when there is a body to replace. A bodiless GET must stay
			// bodiless; injecting one changes upstream semantics.
			if len(raw) > 0 {
				replaced, err := json.Marshal(map[string]interface{}{
					"data":      string(raw),
					"signature": signature,
				})
				if err != nil {
					http.Error(w, fmt.Sprintf("request payload failed during modification: %v", err), http.StatusInternalServerError)
					return
				}

				r.Body = io.NopCloser(bytes.NewReader(replaced))
			}
			gonvoy.Terminal{}.ServeHTTP(w, r)

			resp, err := gonvoy.ResponseBody(r)
			if err != nil {
				return
			}

			raw, err = io.ReadAll(resp)
			if err != nil {
				return
			}

			payload := make(map[string]interface{})
			if err := json.Unmarshal(raw, &payload); err != nil {
				http.Error(w, fmt.Sprintf("response payload failed during unmarshal: %v", err), http.StatusInternalServerError)
				return
			}

			payload["signature"] = signature
			payload["isModified"] = true
			payload["modifiedAt"] = time.Now().UTC().UnixMilli()
			payload["size"] = len(raw)

			changed, err := json.Marshal(payload)
			if err != nil {
				http.Error(w, fmt.Sprintf("response payload failed during modification: %v", err), http.StatusInternalServerError)
				return
			}

			_, _ = w.Write(changed)
		}),
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.WriteRequestBody(),
		gonvoy.WriteResponseBody(),
	)

	envoy.Handle(echoServerFilterName,
		gonvoy.HandlerFunc(func(w gonvoy.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			if err != nil || len(raw) == 0 {
				gonvoy.Terminal{}.ServeHTTP(w, r)
				return
			}

			payload := make(map[string]interface{})
			if err := json.Unmarshal(raw, &payload); err != nil {
				gonvoy.Terminal{}.ServeHTTP(w, r)
				return
			}

			gonvoy.Log(r).Info("request payload should be modified --->", "signature", payload["signature"], "data", payload["data"])
			gonvoy.Terminal{}.ServeHTTP(w, r)
		}),
		gonvoy.OpenBodyAccess(),
		gonvoy.ReadRequestBody(),
	)
}

func main() {}

const (
	bodyReadFilterName   = "http_body_reader"
	bodyWriteFilterName  = "http_body_writer"
	echoServerFilterName = "echoserver"
)
