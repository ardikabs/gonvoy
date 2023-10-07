package envoy

import (
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	responseBody_401 = CreateSimpleJSONBody("UNAUTHORIZED", "UNAUTHORIZED")
	responseBody_403 = CreateSimpleJSONBody("FORBIDDEN", "FORBIDDEN")
	responseBody_500 = CreateSimpleJSONBody("RUNTIME_ERROR", "RUNTIME_ERROR")
)

type ErrorHandler func(Context, error) api.StatusType

func DefaultErrorHandler(ctx Context, err error) api.StatusType {
	if err == nil {
		return api.Continue
	}

	switch unwrapErr := errs.Unwrap(err); unwrapErr {
	case errs.ErrUnauthorized:
		ctx.JSON(http.StatusUnauthorized, responseBody_401, nil, WithLocalReplyDetail(err.Error()))
	case errs.ErrAccessDenied:
		ctx.JSON(http.StatusForbidden, responseBody_403, nil, WithLocalReplyDetail(err.Error()))
	default:
		// hide internal error to end user
		// but printed out the error details to envoy log
		ctx.Log(ErrorLevel, fmt.Sprintf("unidentified error; %v", err))
		ctx.JSON(http.StatusInternalServerError, responseBody_500, map[string]string{"source": "gateway"}, WithLocalReplyDetail(err.Error()))
	}

	return ctx.StatusType()
}
