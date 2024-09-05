package main

import (
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

func (EchoHandler) OnRequestHeader(c gonvoy.Context) error {
	for k, v := range c.Request().Header {
		c.Log().Info("request header --->", k, v)
	}

	return nil
}
