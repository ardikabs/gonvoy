package main

import (
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerOne struct{}

func (h *HandlerOne) RequestHandler(next envoy.RequestHandlerFunc) envoy.RequestHandlerFunc {
	return func(ctx envoy.RequestContext, req *http.Request) error {
		ctx.Add("x-key-id", "0")
		ctx.Add("x-key-id", "1")

		if req.Header.Get("x-error") == "401" {
			return fmt.Errorf("intentionally return unauthorized, %w", errs.ErrUnauthorized)
		}

		ctx.Log(envoy.InfoLevel, "first handler executed")
		ctx.Log(envoy.ErrorLevel, fmt.Sprintln(ctx.Host(), ctx.Path(), ctx.Method(), req.URL.Query()))

		if next == nil {
			return nil
		}

		return next(ctx, req)
	}
}

func (h *HandlerOne) ResponseHandler(next envoy.ResponseHandlerFunc) envoy.ResponseHandlerFunc {
	return func(ctx envoy.ResponseContext, res *http.Response) error {
		ctx.Set("via", "gateway.ardikabs.com")

		if next == nil {
			return nil
		}
		return next(ctx, res)
	}
}
