package envoy

import (
	"errors"
	"net/http"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	ResponseUnauthorized        = NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED")
	ResponseForbidden           = NewMinimalJSONResponse("FORBIDDEN", "FORBIDDEN")
	ResponseTooManyRequest      = NewMinimalJSONResponse("TOO_MANY_REQUEST", "TOO_MANY_REQUEST")
	ResponseInternalServerError = NewMinimalJSONResponse("RUNTIME_ERROR", "RUNTIME_ERROR")
	ResponseServiceUnavailable  = NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE")
)

type ErrorHandler func(Context, error) api.StatusType

func DefaultErrorHandler(ctx Context, err error) api.StatusType {
	unwrapErr := errs.Unwrap(err)
	if unwrapErr == nil {
		return api.Continue
	}

	switch unwrapErr {
	case errs.ErrUnauthorized:
		err = ctx.JSON(
			http.StatusUnauthorized,
			ResponseUnauthorized,
			NewGatewayHeaders(),
			WithResponseCodeDetails(ResponseCodeDetailPrefix_Unauthorized.Wrap(err.Error())))
	case errs.ErrAccessDenied:
		err = ctx.JSON(
			http.StatusForbidden,
			ResponseForbidden,
			NewGatewayHeaders(),
			WithResponseCodeDetails(ResponseCodeDetailPrefix_AccessDenied.Wrap(err.Error())))
	default:
		log := ctx.Log().WithCallDepth(3)
		if errors.Is(err, errs.ErrPanic) {
			log = log.WithCallDepth(1)
		}

		// hide internal error to end user
		// but printed out the error details to envoy log
		log.Error(err, "unidentified error", "host", ctx.Request().Host, "method", ctx.Request().Method, "path", ctx.Request().URL.Path)
		err = ctx.JSON(
			http.StatusInternalServerError,
			ResponseInternalServerError,
			NewGatewayHeaders(),
			WithResponseCodeDetails(ResponseCodeDetailPrefix_Error.Wrap(err.Error())))
	}

	if err != nil {
		return ctx.StatusType()
	}

	return api.LocalReply
}

type HandlerChain interface {
	HandleOnRequestHeader(Context) error
	HandleOnResponseHeader(Context) error
	HandleOnRequestBody(Context) error
	HandleOnResponseBody(Context) error

	SetNext(HandlerChain)
}

type defaultHandlerChain struct {
	handler HttpFilterHandler
	next    HandlerChain
}

func NewHandlerChain(hf HttpFilterHandler) *defaultHandlerChain {
	return &defaultHandlerChain{
		handler: hf,
	}
}

func (b *defaultHandlerChain) HandleOnRequestHeader(c Context) error {
	if err := b.handler.OnRequestHeader(c, c.Request().Header); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnRequestHeader(c)
	}

	return nil
}

func (b *defaultHandlerChain) HandleOnResponseHeader(c Context) error {
	if err := b.handler.OnResponseHeader(c, c.Request().Header); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnResponseHeader(c)
	}

	return nil
}

func (b *defaultHandlerChain) HandleOnRequestBody(c Context) error {
	if err := b.handler.OnRequestBody(c, c.RequestBodyWriter().Bytes()); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnRequestBody(c)
	}

	return nil
}

func (b *defaultHandlerChain) HandleOnResponseBody(c Context) error {
	if err := b.handler.OnResponseBody(c, c.ResponseBodyWriter().Bytes()); err != nil {
		return err
	}

	if c.Committed() {
		return nil
	}

	if b.next != nil {
		return b.next.HandleOnResponseBody(c)
	}

	return nil
}

func (b *defaultHandlerChain) SetNext(hc HandlerChain) {
	b.next = hc
}
