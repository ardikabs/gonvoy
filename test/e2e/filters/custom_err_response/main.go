package main

import (
	"net/http"

	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "custom_err_response"
}

func (Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
	ctrl.AddHandler(Handler{})
	return nil
}

func (Filter) OnComplete(c gaetway.Context) error {
	return nil
}

type Handler struct {
	gaetway.PassthroughHttpFilterHandler
}

func (h Handler) OnResponseHeader(c gaetway.Context) error {
	switch code := c.Response().StatusCode; code {
	case http.StatusUnauthorized:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		rcdetails := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)

		return c.JSON(code, gaetway.NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(rcdetails))

	case http.StatusTooManyRequests:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		rcdetails := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)

		return c.JSON(code, gaetway.NewMinimalJSONResponse("TOO_MANY_REQUESTS", "TOO_MANY_REQUESTS"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(rcdetails))

	case http.StatusServiceUnavailable:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		rcdetails := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)

		return c.JSON(code,
			gaetway.NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(rcdetails))
	}

	return nil
}
