package gonvoy

import (
	"errors"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// HttpFilterHandler represents an interface for an HTTP filter handler.
// In a typical HTTP flow, the sequence of events can be as follows:
// OnRequestHeader -> OnRequestBody -> <Any number of intermediate Envoy processes> -> OnResponseHeader -> OnResponseBody
// HttpFilterHandler is an interface that defines the methods to handle HTTP filter operations.
type HttpFilterHandler interface {
	// Disable disables the HTTP filter handler.
	//
	// It returns a boolean value indicating whether the HTTP filter handler is disabled.
	Disable() bool

	// OnRequestHeader is called when processing the HTTP request header during the OnRequestBody phase.
	//
	OnRequestHeader(c Context) error

	// OnRequestBody is called when processing the HTTP request body during the OnRequestBody phase.
	//
	OnRequestBody(c Context) error

	// OnResponseHeader is called when processing the HTTP response header during the OnResponseHeader phase.
	//
	OnResponseHeader(c Context) error

	// OnResponseBody is called when processing the HTTP response body during the OnResponseBody phase.
	//
	OnResponseBody(c Context) error
}

var (
	responseUnauthorized        = NewMinimalJSONResponse("UNAUTHORIZED", "Unauthorized")
	responseForbidden           = NewMinimalJSONResponse("FORBIDDEN", "Forbidden")
	responseClientClosedRequest = NewMinimalJSONResponse("CLIENT_CLOSED_REQUEST", "Client Closed Request")
	responseRuntimeError        = NewMinimalJSONResponse("RUNTIME_ERROR", "Runtime Error")
	responseBadGateway          = NewMinimalJSONResponse("BAD_GATEWAY", "Bad Gateway")
)

// ErrorHandler is a function type that handles errors in the HTTP filter.
type ErrorHandler func(Context, error) api.StatusType

// DefaultErrorHandler is a default error handler if no custom error handler is provided.
func DefaultErrorHandler(c Context, err error) api.StatusType {
	if err == nil {
		return api.Continue
	}

	host := MustGetProperty(c, "request.host", "-")
	method := MustGetProperty(c, "request.method", "-")
	path := MustGetProperty(c, "request.path", "-")
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

var _ HttpFilterHandler = PassthroughHttpFilterHandler{}

type PassthroughHttpFilterHandler struct{}

func (PassthroughHttpFilterHandler) Disable() bool                    { return false }
func (PassthroughHttpFilterHandler) OnRequestHeader(c Context) error  { return nil }
func (PassthroughHttpFilterHandler) OnRequestBody(c Context) error    { return nil }
func (PassthroughHttpFilterHandler) OnResponseHeader(c Context) error { return nil }
func (PassthroughHttpFilterHandler) OnResponseBody(c Context) error   { return nil }
