package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ardikabs/gonvoy"
)

const bodyWriteFilterName = "http_body_writer"

type BodyWriteFilter struct{}

func (BodyWriteFilter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	cfg, ok := c.GetFilterConfig().(*Config)
	if !ok {
		return fmt.Errorf("unexpected configuration type %T, expecting %T", c.GetFilterConfig(), cfg)
	}

	if !cfg.EnableWrite {
		return nil
	}

	ctrl.AddHandler(&BodyWriteFilterHandler{})
	return nil
}

func (BodyWriteFilter) OnComplete(c gonvoy.Context) error {
	return nil
}

type BodyWriteFilterHandler struct {
	gonvoy.PassthroughHttpFilterHandler

	signature string
}

func (h *BodyWriteFilterHandler) OnRequestHeader(c gonvoy.Context) error {
	header := c.Request().Header

	if v := header.Get("x-modify-body"); v != "true" {
		return c.SkipNextPhase()
	}

	h.signature = header.Get("x-signature")
	if h.signature == "" {
		return fmt.Errorf("signature can not be empty, %w", gonvoy.ErrBadRequest)
	}

	return nil
}

func (h *BodyWriteFilterHandler) OnRequestBody(c gonvoy.Context) error {
	body := c.RequestBody()

	newPayload := make(map[string]interface{})
	newPayload["data"] = body.String()
	newPayload["signature"] = h.signature

	encoder := json.NewEncoder(c.RequestBody())
	if err := encoder.Encode(newPayload); err != nil {
		return fmt.Errorf("request payload failed during modification: %w", err)
	}

	return nil
}

func (h *BodyWriteFilterHandler) OnResponseBody(c gonvoy.Context) error {
	body := c.ResponseBody()

	payload := make(map[string]interface{})
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		return fmt.Errorf("response payload failed during unmarshal: %w", err)
	}

	payload["signature"] = h.signature
	payload["isModified"] = true
	payload["modifiedAt"] = time.Now().UTC().UnixMilli()
	payload["size"] = len(body.Bytes())
	encoder := json.NewEncoder(c.ResponseBody())
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("response payload failed during modification: %w", err)
	}

	return nil
}
