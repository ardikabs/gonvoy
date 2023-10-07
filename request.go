package envoy

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/ardikabs/go-envoy/pkg/types"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type RequestHandlerFunc func(ctx RequestContext, req *http.Request) error

type RequestHandler func(next RequestHandlerFunc) RequestHandlerFunc

var _ RequestContext = &reqContext{}

type reqContext struct {
	api.RequestHeaderMap

	callback   api.FilterCallbacks
	statusType api.StatusType
}

func NewRequestContext(header api.RequestHeaderMap, callback api.FilterCallbacks) (RequestContext, error) {
	if header == nil {
		return nil, errors.New("header MUST not nil")
	}

	if callback == nil {
		return nil, errors.New("callback MUST not nil")
	}

	return &reqContext{
		RequestHeaderMap: header,
		callback:         callback,
		statusType:       api.Continue,
	}, nil
}

func (c *reqContext) Log(lvl LogLevel, msg string) {
	c.callback.Log(api.LogType(lvl), msg)
}

func (c *reqContext) JSON(statusCode int, body []byte, headers map[string]string, opts ...LocalReplyOption) error {
	options := GetDefaultLocalReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	if body == nil {
		body = []byte("{}")
	}

	headers["content-type"] = "application/json"
	c.callback.SendLocalReply(statusCode, string(body), headers, options.grpcStatusCode, options.localReplyDetail)
	c.statusType = options.statusType

	runtime.GC()
	return nil
}

func (c *reqContext) PlainText(statusCode int, body string, opts ...LocalReplyOption) error {
	options := GetDefaultLocalReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	c.callback.SendLocalReply(statusCode, body, map[string]string{}, options.grpcStatusCode, options.localReplyDetail)
	c.statusType = api.LocalReply
	return nil
}

func (c *reqContext) StatusType() api.StatusType {
	return c.statusType
}

type requestHandler struct {
	ctx          RequestContext
	errorHandler ErrorHandler
	reqHandlers  []RequestHandler
}

func NewRequestHandler(ctx RequestContext) *requestHandler {
	return NewRequestHandlerWithErrorHandler(ctx, DefaultErrorHandler)
}

func NewRequestHandlerWithErrorHandler(ctx RequestContext, errHandler ErrorHandler) *requestHandler {
	return &requestHandler{
		ctx:          ctx,
		errorHandler: errHandler,
	}
}

func (h *requestHandler) Use(handler RequestHandler) {
	if handler == nil {
		return
	}

	h.reqHandlers = append(h.reqHandlers, handler)
}

func (h *requestHandler) Handle() (status api.StatusType) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occured; %v, %w", r, errs.ErrInternalServer)
		}

		status = h.ctx.StatusType()

		if err != nil {
			status = h.errorHandler(h.ctx, err)
		}

		return
	}()

	if len(h.reqHandlers) == 0 {
		return
	}

	stack := h.outerHandler
	for i := len(h.reqHandlers) - 1; i >= 0; i-- {
		stack = h.decorateHandler(h.reqHandlers[i](stack))
	}

	req, err := types.NewRequest(h.ctx.Method(), h.ctx.Host(),
		types.WithRequestURI(h.ctx.Path()),
		types.WithRequestHeaderRangeSetter(h.ctx),
	)
	if err != nil {
		err = fmt.Errorf("%v, %w", err, errs.ErrInternalServer)
		return
	}

	err = stack(h.ctx, req)
	return
}

func (h *requestHandler) decorateHandler(next RequestHandlerFunc) RequestHandlerFunc {
	return func(ctx RequestContext, req *http.Request) error {
		if ctx.StatusType() == api.LocalReply {
			return nil
		}

		if err := next(ctx, req); err != nil {
			return err
		}

		return nil
	}
}

func (h *requestHandler) outerHandler(ctx RequestContext, req *http.Request) error {
	return nil
}
