package main

import (
	"net/http"

	"github.com/ardikabs/go-envoy"
)

type HandlerThree struct {
	RequestHeaders map[string]string
}

func (h *HandlerThree) RequestHandler(next envoy.RequestHandlerFunc) envoy.RequestHandlerFunc {
	return func(ctx envoy.RequestContext, req *http.Request) error {
		for k, v := range h.RequestHeaders {
			ctx.Set(k, v)
		}

		if next == nil {
			return nil
		}
		return next(ctx, req)
	}
}

func (h *HandlerThree) ResponseHandler(next envoy.ResponseHandlerFunc) envoy.ResponseHandlerFunc {
	return func(ctx envoy.ResponseContext, res *http.Response) error {
		switch sc := res.StatusCode; sc {
		case http.StatusUnauthorized:
			return ctx.JSON(sc, envoy.CreateSimpleJSONBody("UNAUTHORIZED", "UNAUTHORIZED"), envoy.ToFlatHeader(res.Header))
		case http.StatusTooManyRequests:
			return ctx.JSON(sc, envoy.CreateSimpleJSONBody("TOO_MANY_REQUESTS", "TOO_MANY_REQUESTS"), envoy.ToFlatHeader(res.Header), envoy.WithLocalReplyDetail("rate limit exceeded"))
		case http.StatusServiceUnavailable:
			return ctx.JSON(sc, envoy.CreateSimpleJSONBody("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE"), envoy.ToFlatHeader(res.Header), envoy.WithLocalReplyDetail("service unavailable"))
		}

		if next == nil {
			return nil
		}
		return next(ctx, res)
	}
}
