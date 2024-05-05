package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerThree struct {
	envoy.PassthroughHttpFilterHandler
	RequestHeaders map[string]string
}

func (h *HandlerThree) OnRequestHeader(c envoy.Context, header http.Header) error {
	log := c.Log().WithName("handlerThree")

	for k, v := range h.RequestHeaders {
		c.RequestHeader().Set(k, v)
	}

	log.Info("handling request", "request", c.RequestHeader().AsMap())
	return nil
}

func (h *HandlerThree) OnResponseHeader(c envoy.Context, header http.Header) error {
	switch sc := c.Response().StatusCode; sc {
	case http.StatusUnauthorized:
		return c.JSON(sc,
			envoy.NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED"),
			envoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			envoy.WithResponseCodeDetails(envoy.MustGetProperty(c, "response.code_details", envoy.DefaultResponseCodeDetails)))
	case http.StatusTooManyRequests:
		return c.JSON(sc,
			envoy.NewMinimalJSONResponse("TOO_MANY_REQUESTS", "TOO_MANY_REQUESTS"),
			envoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			envoy.WithResponseCodeDetails(envoy.MustGetProperty(c, "response.code_details", envoy.DefaultResponseCodeDetails)))
	case http.StatusServiceUnavailable:
		return c.JSON(sc,
			envoy.NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE"),
			envoy.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader()),
			envoy.WithResponseCodeDetails(envoy.MustGetProperty(c, "response.code_details", envoy.DefaultResponseCodeDetails)))

	}
	return nil
}

func (h *HandlerThree) OnRequestBody(c envoy.Context, body []byte) error {
	reqBody := make(map[string]interface{})

	if err := json.Unmarshal(body, &reqBody); err != nil {
		return errs.ErrBadRequest
	}

	reqBody["newData"] = "newValue"
	reqBody["handlerName"] = "HandlerThree"
	reqBody["phase"] = "HTTPRequest"

	enc := json.NewEncoder(c.RequestBody())
	return enc.Encode(reqBody)
}

func (h *HandlerThree) OnResponseBody(c envoy.Context, body []byte) error {
	if ct := c.Response().Header.Get(envoy.HeaderContentType); !strings.Contains(ct, "application/json") {
		return nil
	}

	respBody := make(map[string]interface{})

	if err := json.Unmarshal(body, &respBody); err != nil {
		return errs.ErrBadRequest
	}

	respBody["newData"] = "newValue"
	respBody["handlerName"] = "HandlerThree"
	respBody["phase"] = "HTTPResponse"

	enc := json.NewEncoder(c.ResponseBody())
	return enc.Encode(respBody)
}
