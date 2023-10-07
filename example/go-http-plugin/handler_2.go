package main

import (
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerTwo struct{}

func (h *HandlerTwo) RequestHandler(next envoy.RequestHandlerFunc) envoy.RequestHandlerFunc {
	return func(ctx envoy.RequestContext, req *http.Request) error {
		ctx.Add("x-user-id", "0")
		ctx.Add("x-user-id", "1")

		if req.Header.Get("x-error") == "403" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		if req.Header.Get("x-error") == "429" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		if req.Header.Get("x-error") == "503" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		ctx.Log(envoy.InfoLevel, "second handler executed")
		return next(ctx, req)
	}
}

func (h *HandlerTwo) ResponseHandler(next envoy.ResponseHandlerFunc) envoy.ResponseHandlerFunc {
	return func(ctx envoy.ResponseContext, res *http.Response) error {
		ctx.Set("from", "gateway.ardikabs.com/v0")

		return next(ctx, res)
	}
}
