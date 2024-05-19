package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(new(Filter), gonvoy.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "custom_err_response"
}

func (Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	ctrl.AddHandler(Handler{})
	return nil
}

func (Filter) OnComplete(c gonvoy.Context) error {
	return nil
}

type Handler struct {
	gonvoy.PassthroughHttpFilterHandler
}

func (h Handler) OnResponseHeader(c gonvoy.Context, header http.Header) error {
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
