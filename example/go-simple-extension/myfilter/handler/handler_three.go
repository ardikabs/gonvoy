package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ardikabs/gaetway"
)

type HandlerThree struct {
	gaetway.PassthroughHttpFilterHandler
	RequestHeaders map[string]string
}

func (h *HandlerThree) OnRequestHeader(c gaetway.Context) error {
	log := c.Log().WithName("handlerThree")

	for k, v := range h.RequestHeaders {
		c.RequestHeader().Set(k, v)
	}

	data := new(globaldata)
	if ok, err := c.GetCache().Load(GLOBAL, &data); ok && err == nil {
		data.Time3 = time.Now()
		log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
		c.GetCache().Store(GLOBAL, data)
	}

	log.Info("handling request", "request", c.RequestHeader().Export())
	return nil
}

func (h *HandlerThree) OnRequestBody(c gaetway.Context) error {
	if ct := c.Request().Header.Get(gaetway.HeaderContentType); !strings.Contains(ct, gaetway.MIMEApplicationJSON) {
		c.Log().Info("payload size", "size", len(c.RequestBody().Bytes()))
		return nil
	}

	reqBody := make(map[string]interface{})
	if err := json.Unmarshal(c.RequestBody().Bytes(), &reqBody); err != nil {
		return gaetway.ErrBadRequest
	}

	reqBody["newData"] = "newValue"
	reqBody["handlerName"] = "HandlerThree"
	reqBody["phase"] = "HTTPRequest"

	if c.IsRequestBodyWritable() {
		enc := json.NewEncoder(c.RequestBody())
		return enc.Encode(reqBody)
	}

	b, err := json.MarshalIndent(reqBody, "", "    ")
	if err != nil {
		return gaetway.ErrBadRequest
	}

	c.Log().Info("check request body", "payload", string(b))

	return nil
}

func (h *HandlerThree) OnResponseHeader(c gaetway.Context) error {
	switch sc := c.Response().StatusCode; sc {
	case http.StatusUnauthorized:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		details := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)
		return c.JSON(sc,
			gaetway.NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(details))

	case http.StatusTooManyRequests:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		details := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)
		return c.JSON(sc,
			gaetway.NewMinimalJSONResponse("TOO_MANY_REQUESTS", "TOO_MANY_REQUESTS"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(details))

	case http.StatusServiceUnavailable:
		headers := gaetway.NewGatewayHeadersWithEnvoyHeader(c.ResponseHeader())
		details := gaetway.MustGetProperty(c, "response.code_details", gaetway.DefaultResponseCodeDetails)
		return c.JSON(sc,
			gaetway.NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE"),
			gaetway.LocalReplyWithHTTPHeaders(headers),
			gaetway.LocalReplyWithRCDetails(details))
	}

	return nil
}

func (h *HandlerThree) OnResponseBody(c gaetway.Context) error {
	if ct := c.Response().Header.Get(gaetway.HeaderContentType); !strings.Contains(ct, gaetway.MIMEApplicationJSON) {
		return nil
	}

	respBody := make(map[string]interface{})
	if err := json.Unmarshal(c.ResponseBody().Bytes(), &respBody); err != nil {
		c.Log().Error(err, fmt.Sprintf("expecting data type %T, got %v", respBody, string(c.ResponseBody().Bytes())))
		c.Log().Info("skipping response body manipulation ...")
		return nil
	}

	respBody["newData"] = "newValue"
	respBody["handlerName"] = "HandlerThree"
	respBody["phase"] = "HTTPResponse"

	enc := json.NewEncoder(c.ResponseBody())
	return enc.Encode(respBody)
}
