package gonvoy

import (
	"errors"
	"net/http"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterHandler interface {
	Disable() bool

	OnRequestHeader(c Context, header http.Header) error
	OnRequestBody(c Context, body []byte) error
	OnResponseHeader(c Context, header http.Header) error
	OnResponseBody(c Context, body []byte) error
}

type httpFilterProcessor interface {
	HandleOnRequestHeader(Context) error
	HandleOnResponseHeader(Context) error
	HandleOnRequestBody(Context) error
	HandleOnResponseBody(Context) error

	SetNext(httpFilterProcessor)
}

type defaultHttpFilterProcessor struct {
	handler HttpFilterHandler
	next    httpFilterProcessor
}

func NewHttpFilterProcessor(hf HttpFilterHandler) *defaultHttpFilterProcessor {
	return &defaultHttpFilterProcessor{
		handler: hf,
	}
}

func (b *defaultHttpFilterProcessor) HandleOnRequestHeader(c Context) error {
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

func (b *defaultHttpFilterProcessor) HandleOnResponseHeader(c Context) error {
	if err := b.handler.OnResponseHeader(c, c.Response().Header); err != nil {
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

func (b *defaultHttpFilterProcessor) HandleOnRequestBody(c Context) error {
	if err := b.handler.OnRequestBody(c, c.RequestBody().Bytes()); err != nil {
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

func (b *defaultHttpFilterProcessor) HandleOnResponseBody(c Context) error {
	if err := b.handler.OnResponseBody(c, c.ResponseBody().Bytes()); err != nil {
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

func (b *defaultHttpFilterProcessor) SetNext(hfp httpFilterProcessor) {
	b.next = hfp
}

var (
	ResponseUnauthorized        = NewMinimalJSONResponse("UNAUTHORIZED", "UNAUTHORIZED")
	ResponseForbidden           = NewMinimalJSONResponse("FORBIDDEN", "FORBIDDEN")
	ResponseTooManyRequest      = NewMinimalJSONResponse("TOO_MANY_REQUEST", "TOO_MANY_REQUEST")
	ResponseInternalServerError = NewMinimalJSONResponse("RUNTIME_ERROR", "RUNTIME_ERROR")
	ResponseServiceUnavailable  = NewMinimalJSONResponse("SERVICE_UNAVAILABLE", "SERVICE_UNAVAILABLE")
)

type ErrorHandler func(Context, error) api.StatusType

func DefaultErrorHttpFilterHandler(ctx Context, err error) api.StatusType {
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
			WithResponseCodeDetails(DefaultResponseCodeDetailUnauthorized.Wrap(err.Error())))

	case errs.ErrAccessDenied:
		err = ctx.JSON(
			http.StatusForbidden,
			ResponseForbidden,
			NewGatewayHeaders(),
			WithResponseCodeDetails(DefaultResponseCodeDetailAccessDenied.Wrap(err.Error())))

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
			WithResponseCodeDetails(DefaultResponseCodeDetailError.Wrap(err.Error())))
	}

	if err != nil {
		return ctx.StatusType()
	}

	return api.LocalReply
}
