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
	envoy.HandleEcho(bodyReadFilterName,
		func(c gonvoy.EchoContext) error {
			cfg, err := gonvoy.EchoConfig[*Config](c)
			if err != nil || !cfg.EnableRead {
				return nil
			}

			if secretName := c.Header("x-get-secret"); secretName != "" {
				if value, ok := c.Secret(secretName); ok {
					c.SetHeader("x-secret-value", value)
				}

				return nil
			}

			if v := c.Header("x-inspect-body"); v != "true" {
				return c.Next()
			}

			tryWriteOnRequest := c.Header("x-try-write") == "request"
			tryWriteOnResponse := c.Header("x-try-write") == "response"

			body, err := io.ReadAll(c.Body())
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

			if err := c.Next(); err != nil {
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

				code := c.Status()
				if code == 0 {
					code = http.StatusOK
				}

				return c.JSON(code, json.RawMessage(changed))
			}

			return nil
		},
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.ReadRequestBody(),
		gonvoy.ReadResponseBody(),
	)

	envoy.HandleEcho(bodyWriteFilterName,
		func(c gonvoy.EchoContext) error {
			cfg, err := gonvoy.EchoConfig[*Config](c)
			if err != nil || !cfg.EnableWrite {
				return nil
			}

			if v := c.Header("x-modify-body"); v != "true" {
				return c.Next()
			}

			signature := c.Header("x-signature")
			if signature == "" {
				return c.String(http.StatusBadRequest, "signature can not be empty")
			}

			raw, err := io.ReadAll(c.Body())
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

			if err := c.Next(); err != nil {
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

			code := c.Status()
			if code == 0 {
				code = http.StatusOK
			}

			return c.JSON(code, json.RawMessage(changed))
		},
		gonvoy.WithConfig(new(Config)),
		gonvoy.OpenBodyAccess(),
		gonvoy.WriteRequestBody(),
		gonvoy.WriteResponseBody(),
	)

	envoy.HandleEcho(echoServerFilterName,
		func(c gonvoy.EchoContext) error {
			raw, err := io.ReadAll(c.Body())
			if err != nil || len(raw) == 0 {
				return c.Next()
			}

			payload := make(map[string]interface{})
			if err := json.Unmarshal(raw, &payload); err != nil {
				return c.Next()
			}

			c.Log().Info("request payload should be modified --->", "signature", payload["signature"], "data", payload["data"])

			return c.Next()
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
