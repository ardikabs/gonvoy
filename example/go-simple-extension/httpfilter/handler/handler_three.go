package handler

import (
	"net/http"

	"github.com/ardikabs/go-envoy"
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
