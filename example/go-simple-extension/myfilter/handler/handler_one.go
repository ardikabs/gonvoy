package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/errs"
)

type HandlerOne struct {
	gonvoy.PassthroughHttpFilterHandler
}

func (h *HandlerOne) OnRequestHeader(c gonvoy.Context) error {
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
			return c.JSON(http.StatusOK, gonvoy.NewMinimalJSONResponse("SUCCESS", "SUCCESS"), nil)
		}(); err != nil {
			return err
		}
	}

	if c.Request().Header.Get("x-skip-next-phase") == "true" {
		return c.SkipNextPhase()
	}

	if c.Request().Header.Get("x-data") == "global" {
		data := new(globaldata)
		data.Name = "from-handler-one"

		if ok, err := c.GetCache().Load(GLOBAL, &data); ok && err == nil {
			data.Time = time.Now()
			log.Info("got existing global data", "data", data, "pointer", fmt.Sprintf("%p", data))
		}

		c.GetCache().Store(GLOBAL, data)
	}

	if c.Request().Header.Get("x-error") == "panick" {
		panicNilMapOuter()
	}

	log.Error(errors.New("error from handler one"), "handling request", "host", c.Request().Host, "path", c.Request().URL.Path, "method", c.Request().Method, "query", c.Request().URL.Query())
	return nil
}

func (h *HandlerOne) OnResponseHeader(c gonvoy.Context) error {
	c.ResponseHeader().Set("via", "gateway.ardikabs.com")
	return nil
}

func panicNilMapOuter() {
	panicNilMapInner()
}

// nolint:nilness
func panicNilMapInner() {
	var a map[string]string
	a["blbl"] = "sdasd"
}
