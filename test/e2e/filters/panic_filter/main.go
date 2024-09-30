package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.RegisterHttpFilter(
		panicFilterName,
		func() gonvoy.HttpFilter {
			return new(Filter)
		},
		gonvoy.ConfigOptions{},
	)
}

func main() {}

const panicFilterName = "panic_filter"

type Filter struct{}

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
