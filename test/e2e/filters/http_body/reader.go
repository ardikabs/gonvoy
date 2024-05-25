package main

import (
	"encoding/json"
	"fmt"

	"github.com/ardikabs/gonvoy"
)

type BodyReadFilter struct{}

func (BodyReadFilter) Name() string {
	return "http_body_reader"
}

func (BodyReadFilter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	cfg, ok := c.GetFilterConfig().(*Config)
	if !ok {
		return fmt.Errorf("unexpected configuration type %T, expecting %T", c.GetFilterConfig(), cfg)
	}

	if !cfg.EnableRead {
		return nil
	}

	ctrl.AddHandler(&BodyReadFilterHandler{})
	return nil
}

func (BodyReadFilter) OnComplete(c gonvoy.Context) error {
	return nil
}

type BodyReadFilterHandler struct {
	gonvoy.PassthroughHttpFilterHandler

	tryWriteOnRequest  bool
	tryWriteOnResponse bool
}

func (h *BodyReadFilterHandler) OnRequestHeader(c gonvoy.Context) error {
	header := c.Request().Header

	if v := header.Get("x-inspect-body"); v != "true" {
		return c.SkipNextPhase()
	}

	if v := header.Get("x-try-write"); v == "request" {
		h.tryWriteOnRequest = true
	}

	if v := header.Get("x-try-write"); v == "response" {
		h.tryWriteOnResponse = true
	}

	return nil
}

func (h *BodyReadFilterHandler) OnRequestBody(c gonvoy.Context) error {
	body := c.RequestBody()

	c.Log().Info("request body payload --->", "data", body.Bytes(), "size", len(body.Bytes()), "mode", "READ")

	if h.tryWriteOnRequest {
		newPayload := make(map[string]interface{})
		newPayload["data"] = body.String()

		encoder := json.NewEncoder(body)
		if err := encoder.Encode(newPayload); err != nil {
			return err
		}
	}

	return nil
}

func (h *BodyReadFilterHandler) OnResponseBody(c gonvoy.Context) error {
	body := c.ResponseBody()

	payload := make(map[string]interface{})
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		return fmt.Errorf("response payload failed during unmarshal: %w", err)
	}

	c.Log().Info("response body payload --->", "state", payload["state"], "mode", "READ")

	if h.tryWriteOnResponse {
		payload["data"] = body.String()

		encoder := json.NewEncoder(body)
		if err := encoder.Encode(payload); err != nil {
			return err
		}
	}

	return nil
}
