package main

import (
	"fmt"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerTwo struct{}

func (h *HandlerTwo) RequestHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.RequestHeader().Add("x-user-id", "0")
		c.RequestHeader().Add("x-user-id", "1")

		if c.Request().Header.Get("x-error") == "403" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		if c.Request().Header.Get("x-error") == "429" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		if c.Request().Header.Get("x-error") == "503" {
			return fmt.Errorf("intentionally return forbidden, %w", errs.ErrAccessDenied)
		}

		return next(c)
	}
}

func (h *HandlerTwo) ResponseHandler(next envoy.HandlerFunc) envoy.HandlerFunc {
	return func(c envoy.Context) error {
		c.RequestHeader().Set("from", "gateway.ardikabs.com/v0")

		return next(c)
	}
}
