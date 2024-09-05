package main

import (
	"net/http"

	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{
		AutoReloadRoute: true,
	})
}

func main() {}

type Filter struct{}

func (Filter) Name() string {
	return "http_reroute"
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
