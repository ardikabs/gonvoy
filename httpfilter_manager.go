package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterHandlerManager interface {
	SetErrorHandler(ErrorHandler)
	RegisterHandler(HttpFilterHandler)
	Serve(Context, HttpFilterPhase) api.StatusType
}

type HttpFilterPhase uint

const (
	OnRequestHeaderPhase HttpFilterPhase = iota + 1
	OnResponseHeaderPhase
	OnRequestBodyPhase
	OnResponseBodyPhase
)

type DefaultHttpFilterHandlerManager struct {
	errorHandler ErrorHandler
	first        httpFilterProcessor
	last         httpFilterProcessor
}

func (h *DefaultHttpFilterHandlerManager) SetErrorHandler(handler ErrorHandler) {
	if handler == nil {
		return
	}

	h.errorHandler = handler
}

func (h *DefaultHttpFilterHandlerManager) RegisterHandler(handler HttpFilterHandler) {
	if util.IsNil(handler) || handler.Disable() {
		return
	}

	processor := NewHttpFilterProcessor(handler)
	if h.first == nil {
		h.first = processor
		h.last = processor
		return
	}

	h.last.SetNext(processor)
	h.last = processor
}

func (h *DefaultHttpFilterHandlerManager) Serve(c Context, phase HttpFilterPhase) (status api.StatusType) {
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
