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
		gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
			return gonvoy.Handler(func(c gonvoy.Context) error {
				cfg, err := gonvoy.ConfigOf[*Config](c)
				if err != nil || !cfg.EnableRead {
					return nil
				}

				if secretName := c.Request().Header.Get("x-get-secret"); secretName != "" {
					if value, ok := c.Secret(secretName); ok {
						c.Response().Header.Set("x-secret-value", value)
					}

					return nil
				}

				if v := c.Request().Header.Get("x-inspect-body"); v != "true" {
					return nil
				}

				tryWriteOnRequest := c.Request().Header.Get("x-try-write") == "request"
				tryWriteOnResponse := c.Request().Header.Get("x-try-write") == "response"

				body, err := io.ReadAll(c.RequestBody())
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
				}

				c.Log().Info("request body payload --->", "data", string(body), "size", len(body), "mode", "READ")

				if tryWriteOnRequest {
					newPayload := map[string]interface{}{"data": string(body)}

					replaced, err := json.Marshal(newPayload)
					if err != nil {
						return c.String(http.StatusInternalServerError, err.Error())
					}

					c.Request().Body = io.NopCloser(bytes.NewReader(replaced))
				}

				if err := next(c); err != nil {
					return err
				}

				resp, err := c.ResponseBody()
				if err != nil {
					return nil
				}

				raw, err := io.ReadAll(resp)
				if err != nil {
					return nil
				}

				payload := make(map[string]interface{})
				if err := json.Unmarshal(raw, &payload); err != nil {
					return c.String(http.StatusInternalServerError, fmt.Sprintf("response payload failed during unmarshal: %v", err))
				}

				c.Log().Info("response body payload --->", "state", payload["state"], "mode", "READ")

				if tryWriteOnResponse {
					payload["data"] = string(raw)

					changed, err := json.Marshal(payload)
					if err != nil {
						return nil
					}

					_, _ = c.Write(changed)

					return nil
				}

				return nil
			})
		}),
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.ReadRequestBody(),
		gonvoy.ReadResponseBody(),
	)

	envoy.Handle(bodyWriteFilterName,
		gonvoy.Use(func(next gonvoy.Handler) gonvoy.Handler {
			return gonvoy.Handler(func(c gonvoy.Context) error {
				cfg, err := gonvoy.ConfigOf[*Config](c)
				if err != nil || !cfg.EnableWrite {
					return nil
				}

				if v := c.Request().Header.Get("x-modify-body"); v != "true" {
					return nil
				}

				signature := c.Request().Header.Get("x-signature")
				if signature == "" {
					return c.String(http.StatusBadRequest, "signature can not be empty")
				}

				raw, err := io.ReadAll(c.RequestBody())
				if err != nil {
					return c.String(http.StatusInternalServerError, err.Error())
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
						return c.String(http.StatusInternalServerError, fmt.Sprintf("request payload failed during modification: %v", err))
					}

					c.Request().Body = io.NopCloser(bytes.NewReader(replaced))
				}

				if err := next(c); err != nil {
					return err
				}

				resp, err := c.ResponseBody()
				if err != nil {
					return nil
				}

				raw, err = io.ReadAll(resp)
				if err != nil {
					return nil
				}

				payload := make(map[string]interface{})
				if err := json.Unmarshal(raw, &payload); err != nil {
					return c.String(http.StatusInternalServerError, fmt.Sprintf("response payload failed during unmarshal: %v", err))
				}

				payload["signature"] = signature
				payload["isModified"] = true
				payload["modifiedAt"] = time.Now().UTC().UnixMilli()
				payload["size"] = len(raw)

				changed, err := json.Marshal(payload)
				if err != nil {
					return c.String(http.StatusInternalServerError, fmt.Sprintf("response payload failed during modification: %v", err))
				}

				_, _ = c.Write(changed)

				return nil
			})
		}),
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.WriteRequestBody(),
		gonvoy.WriteResponseBody(),
	)

	envoy.Handle(echoServerFilterName,
		func(c gonvoy.Context) error {
			raw, err := io.ReadAll(c.RequestBody())
			if err != nil || len(raw) == 0 {
				return nil
			}

			payload := make(map[string]interface{})
			if err := json.Unmarshal(raw, &payload); err != nil {
				return nil
			}

			c.Log().Info("request payload should be modified --->", "signature", payload["signature"], "data", payload["data"])

			return nil
		},
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
