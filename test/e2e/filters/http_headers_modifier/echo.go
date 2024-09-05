package main

import (
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

func (EchoHandler) OnRequestHeader(c gaetway.Context) error {
	for k, v := range c.Request().Header {
		c.Log().Info("request header --->", k, v)
	}

	return nil
}
