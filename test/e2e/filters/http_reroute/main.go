package main

import (
	"net/http"

	"github.com/ardikabs/gonvoy"
)

const filterName = "http_reroute"

func init() {
	gonvoy.RunHttpFilter(
		filterName,
		func() gonvoy.HttpFilter {
			return new(Filter)
		},
		gonvoy.ConfigOptions{
			AutoReloadRoute: true,
		},
	)
}

func main() {}

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
	header := c.Request().Header

	if v := header.Get("x-route-to"); v == "staticreply" {
		c.RequestHeader().Set("x-upstream-name", "staticreply")
	}

	if v := header.Get("x-path-changed-to"); v == "staticreply" {
		c.SetRequestPath("/staticreply")
	}

	if v := header.Get("x-changed-host"); v == "true" {
		c.SetRequestHost("staticreply.svc")
		c.SetRequestMethod(http.MethodPost)
	}

	return nil
}
