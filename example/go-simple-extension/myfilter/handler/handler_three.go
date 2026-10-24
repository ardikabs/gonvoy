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

// Three handles the inner layer of the example chain.
func Three(gonvoy.Handler) gonvoy.Handler {
	return gonvoy.Echo(func(c gonvoy.EchoContext) error {
		log := c.Log().WithName("handlerThree")

		var reqHeaders map[string]string
		if cfg, err := gonvoy.EchoConfig[*Config](c); err == nil && cfg != nil {
			reqHeaders = cfg.RequestHeaders
		}

		for k, v := range reqHeaders {
			c.SetRequestHeader(k, v)
		}

		data := new(globaldata)
		if ok, err := c.Cache().Load(GLOBAL, &data); ok && err == nil {
			data.Time3 = time.Now()
			log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
			c.Cache().Store(GLOBAL, data)
		}

		log.Info("handling request", "request", c.Request().Header)

		if ct := c.Header(gonvoy.HeaderContentType); strings.Contains(ct, gonvoy.MIMEApplicationJSON) {
			body, err := io.ReadAll(c.Body())
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}

			reqBody := make(map[string]interface{})
			if err := json.Unmarshal(body, &reqBody); err != nil {
				return badRequest(c, "unexpected payload")
			}

			reqBody["newData"] = "newValue"
			reqBody["handlerName"] = "HandlerThree"
			reqBody["phase"] = "HTTPRequest"

			replaced, err := json.Marshal(reqBody)
			if err != nil {
				return badRequest(c, "unexpected payload")
			}

			c.Request().Body = io.NopCloser(bytes.NewReader(replaced))
		} else {
			log.Info("payload size", "size", len(mustReadAll(c.Request())))
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

		respBody := make(map[string]interface{})
		if err := json.Unmarshal(raw, &respBody); err != nil {
			c.Log().Error(err, fmt.Sprintf("expecting data type %T, got %v", respBody, string(raw)))
			c.Log().Info("skipping response body manipulation ...")
			return nil
		}

		respBody["newData"] = "newValue"
		respBody["handlerName"] = "HandlerThree"
		respBody["phase"] = "HTTPResponse"

		changed, err := json.Marshal(respBody)
		if err != nil {
			return nil
		}

		code := c.Status()
		if code == 0 {
			code = http.StatusOK
		}

		return c.JSON(code, json.RawMessage(changed))
	})
}

func mustReadAll(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	return b
}
