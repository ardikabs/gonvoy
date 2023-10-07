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

type ResponseHandlerFunc func(ctx ResponseContext, res *http.Response) error

type ResponseHandler func(next ResponseHandlerFunc) ResponseHandlerFunc

type resContext struct {
	api.ResponseHeaderMap

	callback   api.FilterCallbacks
	statusType api.StatusType
}

func NewResponseContext(header api.ResponseHeaderMap, callback api.FilterCallbacks) (ResponseContext, error) {
	if header == nil {
		return nil, errors.New("header MUST not nil")
	}

	if callback == nil {
		return nil, errors.New("callback MUST not nil")
	}

	return &resContext{
		ResponseHeaderMap: header,
		callback:          callback,
		statusType:        api.Continue,
	}, nil
}

func (c *resContext) Log(lvl LogLevel, msg string) {
	c.callback.Log(api.LogType(lvl), msg)
}

func (c *resContext) JSON(statusCode int, body []byte, headers map[string]string, opts ...LocalReplyOption) error {
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

func (c *resContext) PlainText(statusCode int, msg string, opts ...LocalReplyOption) error {
	options := GetDefaultLocalReplyOptions()
	for _, opt := range opts {
		opt(options)
	}
	c.callback.SendLocalReply(statusCode, msg, map[string]string{}, options.grpcStatusCode, options.localReplyDetail)
	c.statusType = options.statusType
	return nil
}

func (c *resContext) StatusType() api.StatusType {
	return c.statusType
}

type responseHandler struct {
	ctx          ResponseContext
	errorHandler ErrorHandler
	resHandlers  []ResponseHandler
}

func NewResponseHandler(ctx ResponseContext) *responseHandler {
	return NewResponseHandlerWithErrorHandler(ctx, DefaultErrorHandler)
}

func NewResponseHandlerWithErrorHandler(ctx ResponseContext, errHandler ErrorHandler) *responseHandler {
	return &responseHandler{
		ctx:          ctx,
		errorHandler: errHandler,
	}
}

func (h *responseHandler) Use(handler ResponseHandler) {
	if handler == nil {
		return
	}

	h.resHandlers = append(h.resHandlers, handler)
}

func (h *responseHandler) Handle() (status api.StatusType) {
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

	if len(h.resHandlers) == 0 {
		return
	}

	stack := h.outerHandler
	for i := len(h.resHandlers) - 1; i >= 0; i-- {
		stack = h.decorate(h.resHandlers[i](stack))
	}

	statusCode, ok := h.ctx.Status()
	if !ok {
		statusCode = http.StatusInternalServerError
	}

	res, err := types.NewResponse(statusCode, types.WithResponseHeaderRangeSetter(h.ctx))
	if err != nil {
		err = fmt.Errorf("%v, %w", err, errs.ErrInternalServer)
		return
	}

	err = stack(h.ctx, res)
	return
}

func (h *responseHandler) decorate(next ResponseHandlerFunc) ResponseHandlerFunc {
	return func(ctx ResponseContext, res *http.Response) error {
		if ctx.StatusType() == api.LocalReply {
			return nil
		}

		if err := next(ctx, res); err != nil {
			return err
		}

		return nil
	}
}

func (h *responseHandler) outerHandler(ctx ResponseContext, res *http.Response) error {
	return nil
}
