package main

import (
	"encoding/json"

	"github.com/ardikabs/gaetway"
)

type Echoserver struct{}

func (Echoserver) Name() string {
	return "echoserver"
}

func (Echoserver) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
	ctrl.AddHandler(EchoHandler{})
	return nil
}

func (Echoserver) OnComplete(c gaetway.Context) error {
	return nil
}

type EchoHandler struct {
	gaetway.PassthroughHttpFilterHandler
}

func (EchoHandler) OnRequestBody(c gaetway.Context) error {
	body := c.RequestBody()
	payload := make(map[string]interface{})
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		return nil
	}

	c.Log().Info("request payload should be modified --->", "signature", payload["signature"], "data", payload["data"])

	return nil
}
