package main

import (
	"github.com/ardikabs/gaetway"
)

func init() {
	gonvoy.RunHttpFilter(new(Filter), gonvoy.ConfigOptions{})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "panic_filter"
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

func (h Handler) OnRequestHeader(c gonvoy.Context) error {
	if c.Request().Header.Get("x-panic-at") == "header" {
		panic("panic during request header handling")
	}

	return nil
}

func (h Handler) OnResponseHeader(c gonvoy.Context) error {
	if c.Response().Header.Get("x-panic-at") == "header" {
		panic("panic during response header handling")
	}

	return nil
}
