package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/pkg/errs"
)

type HandlerOne struct {
	envoy.PassthroughHttpFilterHandler
}

func (h *HandlerOne) OnRequestHeader(c envoy.Context, header http.Header) error {
	log := c.Log().WithName("handlerOne").WithName("outer").WithName("inner")

	c.RequestHeader().Add("x-key-id", "0")
	c.RequestHeader().Add("x-key-id", "1")

	if c.Request().Header.Get("x-error") == "401" {
		return fmt.Errorf("intentionally return unauthorized, %w", errs.ErrUnauthorized)
	}

	if c.Request().Header.Get("x-error") == "5xx" {
		return errors.New("intentionally return unidentified error")
	}

	if c.Request().Header.Get("x-error") == "503" {
		return c.String(http.StatusServiceUnavailable, "service unavailable", nil)
	}

	if c.Request().Header.Get("x-error") == "200" {
		if err := func() error {
			return c.JSON(http.StatusOK, envoy.NewMinimalJSONResponse("SUCCESS", "SUCCESS"), nil)
		}(); err != nil {
			return err
		}
	}

	if c.Request().Header.Get("x-error") == "panick" {
		panicNilMapOuter()
	}

	log.Error(errors.New("error from handler one"), "handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
	return nil
}

func (h *HandlerOne) OnResponseHeader(c envoy.Context, header http.Header) error {
	c.ResponseHeader().Set("via", "gateway.ardikabs.com")
	return nil
}

func panicNilMapOuter() {
	panicNilMapInner()
}

func panicNilMapInner() {
	var a map[string]string
	a["blbl"] = "sdasd"
}
