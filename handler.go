package envoy

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HandlerFunc func(ctx Context) error

type Handler func(next HandlerFunc) HandlerFunc

func HandlerDecorator(next HandlerFunc) HandlerFunc {
	return func(c Context) error {
		if c.Committed() {
			// if committed, it means current chaining needs to be halted
			// and return status to the Envoy immediately
			return nil
		}

		return next(c)
	}
}

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

// NewMinimalJSONResponse creates a minimal JSON body as a form of bytes.
func NewMinimalJSONResponse(code, message string, errs ...error) []byte {
	bodyMap := make(map[string]interface{})
	bodyMap["code"] = code
	bodyMap["message"] = message
	bodyMap["errors"] = nil
	bodyMap["data"] = make(map[string]interface{}, 0)
	bodyMap["serverTime"] = time.Now().UnixMilli()

	listErrs := make([]string, len(errs))
	for i, err := range errs {
		listErrs[i] = err.Error()
	}
	bodyMap["errors"] = listErrs

	bodyByte, err := json.Marshal(bodyMap)
	if err != nil {
		bodyByte = []byte("{}")
	}

	return bodyByte
}
