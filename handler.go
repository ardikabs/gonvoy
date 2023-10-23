package envoy

import (
	"net/http"

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
	responseBody_401 = CreateSimpleJSONBody("UNAUTHORIZED", "UNAUTHORIZED")
	responseBody_403 = CreateSimpleJSONBody("FORBIDDEN", "FORBIDDEN")
	responseBody_500 = CreateSimpleJSONBody("RUNTIME_ERROR", "RUNTIME_ERROR")
)

type ErrorHandler func(Context, error) api.StatusType

func DefaultErrorHandler(ctx Context, err error) api.StatusType {
	switch unwrapErr := errs.Unwrap(err); unwrapErr {
	case nil:
		break
	case errs.ErrUnauthorized:
		err = ctx.JSON(http.StatusUnauthorized, responseBody_401, nil, WithResponseCodeDetails(err.Error()))
	case errs.ErrAccessDenied:
		err = ctx.JSON(http.StatusForbidden, responseBody_403, nil, WithResponseCodeDetails(err.Error()))
	default:
		// hide internal error to end user
		// but printed out the error details to envoy log
		ctx.Log().Error(err, "unidentified error", "host", ctx.Request().Host, "method", ctx.Request().Method, "path", ctx.Request().URL.Path)
		err = ctx.JSON(http.StatusInternalServerError, responseBody_500, map[string]string{"reporter": "gateway"}, WithResponseCodeDetails(err.Error()))
	}

	if err != nil {
		return ctx.StatusType()
	}

	return api.LocalReply
}
