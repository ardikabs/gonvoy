package gonvoy

import (
	"errors"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	responseUnauthorized        = NewMinimalJSONResponse("UNAUTHORIZED", "Unauthorized")
	responseForbidden           = NewMinimalJSONResponse("FORBIDDEN", "Forbidden")
	responseClientClosedRequest = NewMinimalJSONResponse("CLIENT_CLOSED_REQUEST", "Client Closed Request")
	responseRuntimeError        = NewMinimalJSONResponse("RUNTIME_ERROR", "Runtime Error")
	responseBadGateway          = NewMinimalJSONResponse("BAD_GATEWAY", "Bad Gateway")
)

// defaultErrorHandler is the default error handler if no custom error handler is provided.
func defaultErrorHandler(c bridgeContext, err error) api.StatusType {
	if err == nil {
		return api.Continue
	}

	host := mustGetProperty(c, "request.host", "-")
	method := mustGetProperty(c, "request.method", "-")
	path := mustGetProperty(c, "request.path", "-")
	log := c.Log().WithValues("host", host, "method", method, "path", path)

	switch {
	case errors.Is(err, ErrUnauthorized):
		log.V(1).Info("request unauthorized", "reason", err.Error())

		err = c.JSON(http.StatusUnauthorized, responseUnauthorized,
			LocalReplyWithHTTPHeaders(NewGatewayHeaders()),
			LocalReplyWithRCDetails(DefaultResponseCodeDetailUnauthorized.Wrap(err.Error())))

	case errors.Is(err, ErrAccessDenied):
		log.V(1).Info("request denied", "reason", err.Error())

		err = c.JSON(http.StatusForbidden, responseForbidden,
			LocalReplyWithHTTPHeaders(NewGatewayHeaders()),
			LocalReplyWithRCDetails(DefaultResponseCodeDetailAccessDenied.Wrap(err.Error())))

	case errors.Is(err, ErrOperationNotPermitted):
		log.V(1).Info("request operation not permitted", "reason", err.Error())

		err = c.JSON(http.StatusBadGateway, responseBadGateway,
			LocalReplyWithHTTPHeaders(NewGatewayHeaders()),
			LocalReplyWithRCDetails(DefaultResponseCodeDetailError.Wrap(err.Error())))

	case errors.Is(err, ErrClientClosedRequest):
		log.V(1).Info("request prematurely closed by client", "reason", err.Error())

		err = c.JSON(499, responseClientClosedRequest,
			LocalReplyWithHTTPHeaders(NewGatewayHeaders()),
			LocalReplyWithRCDetails(DefaultResponseCodeDetailInfo.Wrap(err.Error())),
		)

	default:
		log := c.Log().WithCallDepth(3)
		if errors.Is(err, ErrRuntime) {
			log = log.WithCallDepth(1)
		}

		// hide internal error to end user
		// but printed out the error details to envoy log
		log.Error(err, "unidentified error")

		err = c.JSON(http.StatusInternalServerError, responseRuntimeError,
			LocalReplyWithHTTPHeaders(NewGatewayHeaders()),
			LocalReplyWithRCDetails(DefaultResponseCodeDetailError.Wrap(err.Error())))
	}

	if err != nil {
		log.Error(err, "unexpected error")

		// if we encounter another error, we will ignore the error
		// and allowing the request/response to proceed to the next Envoy filter.
		// Though, this condition is expected to be highly unlikely.
		return api.Continue
	}

	return api.LocalReply
}
