package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterHandlerManager interface {
	SetErrorHandler(ErrorHandler)
	Register(HttpFilterHandler) HttpFilterHandlerManager

	handle(Context, HttpFilterActionPhase) api.StatusType
}

type HttpFilterActionPhase uint

const (
	OnRequestHeaderPhase HttpFilterActionPhase = iota + 1
	OnResponseHeaderPhase
	OnRequestBodyPhase
	OnResponseBodyPhase
)

func newHandlerManager() *handlerManager {
	return &handlerManager{
		errorHandler: DefaultErrorHttpFilterHandler,
	}
}

type handlerManager struct {
	errorHandler ErrorHandler
	first        httpFilterProcessor
	last         httpFilterProcessor
}

func (h *handlerManager) SetErrorHandler(handler ErrorHandler) {
	if handler == nil {
		return
	}

	h.errorHandler = handler
}

func (h *handlerManager) Register(handler HttpFilterHandler) HttpFilterHandlerManager {
	if util.IsNil(handler) || handler.Disable() {
		return h
	}

	processor := NewHttpFilterProcessor(handler)
	if h.first == nil {
		h.first = processor
		h.last = processor
		return h
	}

	h.last.SetNext(processor)
	h.last = processor
	return h
}

func (h *handlerManager) handle(c Context, phase HttpFilterActionPhase) (status api.StatusType) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w; %v", errs.ErrPanic, r)
		}

		if err != nil {
			h.errorHandler(c, err)
		}

		status = c.StatusType()
	}()

	if h.first == nil {
		h.first = NewHttpFilterProcessor(PassthroughHttpFilterHandler{})
	}

	switch phase {
	case OnRequestHeaderPhase:
		err = h.first.HandleOnRequestHeader(c)
	case OnResponseHeaderPhase:
		err = h.first.HandleOnResponseHeader(c)
	case OnRequestBodyPhase:
		err = h.first.HandleOnRequestBody(c)
	case OnResponseBodyPhase:
		err = h.first.HandleOnResponseBody(c)
	}

	return status
}
