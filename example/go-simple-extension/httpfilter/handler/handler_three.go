package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/errs"
)

type HandlerThree struct {
	gonvoy.PassthroughHttpFilterHandler
	RequestHeaders map[string]string
}

func (h *HandlerThree) OnRequestHeader(c gonvoy.Context, header http.Header) error {
	log := c.Log().WithName("handlerThree")

	for k, v := range h.RequestHeaders {
		c.RequestHeader().Set(k, v)
	}

	log.Info("handling request", "request", c.RequestHeader().AsMap())
	return nil
}

func (h *HandlerThree) OnResponseHeader(c gonvoy.Context, header http.Header) error {
	switch sc := c.Response().StatusCode; sc {
	case http.StatusUnauthorized:
		return c.JSON(sc,
			gonvoy.NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED"),
			gonvoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			gonvoy.WithResponseCodeDetails(gonvoy.MustGetProperty(c, "response.code_details", gonvoy.DefaultResponseCodeDetails)))
	case http.StatusTooManyRequests:
		return c.JSON(sc,
			gonvoy.NewMinimalJSONResponse("TOO_MANY_REQUESTS", "TOO_MANY_REQUESTS"),
			gonvoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			gonvoy.WithResponseCodeDetails(gonvoy.MustGetProperty(c, "response.code_details", gonvoy.DefaultResponseCodeDetails)))
	case http.StatusServiceUnavailable:
		return c.JSON(sc,
			gonvoy.NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE"),
			gonvoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			gonvoy.WithResponseCodeDetails(gonvoy.MustGetProperty(c, "response.code_details", gonvoy.DefaultResponseCodeDetails)))

	}
	return nil
}

func (h *HandlerThree) OnRequestBody(c gonvoy.Context, body []byte) error {
	if ct := c.Request().Header.Get(gonvoy.HeaderContentType); !strings.Contains(ct, "application/json") {
		return nil
	}

	reqBody := make(map[string]interface{})
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return errs.ErrBadRequest
	}

	reqBody["newData"] = "newValue"
	reqBody["handlerName"] = "HandlerThree"
	reqBody["phase"] = "HTTPRequest"

	if c.IsRequestBodyWriteable() {
		enc := json.NewEncoder(c.RequestBody())
		return enc.Encode(reqBody)
	}

	b, err := json.MarshalIndent(reqBody, "", "    ")
	if err != nil {
		return errs.ErrBadRequest
	}

	c.Log().Info("check request body", "payload", string(b))
	return nil
}

func (h *HandlerThree) OnResponseBody(c gonvoy.Context, body []byte) error {
	if ct := c.Response().Header.Get(gonvoy.HeaderContentType); !strings.Contains(ct, "application/json") {
		return nil
	}

	respBody := make(map[string]interface{})
	if err := json.Unmarshal(body, &respBody); err != nil {
		c.Log().Error(err, fmt.Sprintf("expecting data type %T, got %v", respBody, string(body)))
		c.Log().Info("skipping response body manipulation ...")
		return nil
	}

	respBody["newData"] = "newValue"
	respBody["handlerName"] = "HandlerThree"
	respBody["phase"] = "HTTPResponse"

	enc := json.NewEncoder(c.ResponseBody())
	return enc.Encode(respBody)
}
