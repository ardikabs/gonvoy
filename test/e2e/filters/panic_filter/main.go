package main

import (
	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "panic_filter"
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

func (h Handler) OnRequestHeader(c gaetway.Context) error {
	if c.Request().Header.Get("x-panic-at") == "header" {
		panic("panic during request header handling")
	}

	return nil
}

func (h Handler) OnResponseHeader(c gaetway.Context) error {
	if c.Response().Header.Get("x-panic-at") == "header" {
		panic("panic during response header handling")
	}

	return nil
}
