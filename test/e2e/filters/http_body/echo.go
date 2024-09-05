package main

import (
	"encoding/json"

	"github.com/ardikabs/gaetway"
)

type Echoserver struct{}

func (Echoserver) Name() string {
	return "echoserver"
}

func (Echoserver) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	ctrl.AddHandler(EchoHandler{})
	return nil
}

func (Echoserver) OnComplete(c gonvoy.Context) error {
	return nil
}

type EchoHandler struct {
	gonvoy.PassthroughHttpFilterHandler
}

func (EchoHandler) OnRequestBody(c gonvoy.Context) error {
	body := c.RequestBody()
	payload := make(map[string]interface{})
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		return nil
	}

	c.Log().Info("request payload should be modified --->", "signature", payload["signature"], "data", payload["data"])

	return nil
}
