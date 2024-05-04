package envoy

import (
	"fmt"

	"github.com/ardikabs/go-envoy/pkg/errs"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HandlerManager interface {
	SetErrorHandler(ErrorHandler)
	Use(HttpFilterHandler) HandlerManager

	handle(Context, ActionPhase) api.StatusType
}

type ActionPhase uint

const (
	OnRequestHeaderPhase ActionPhase = iota + 1
	OnResponseHeaderPhase
	OnRequestBodyPhase
	OnResponseBodyPhase
)

func newHandlerManager() *handlerManager {
	return &handlerManager{
		errorHandler: DefaultErrorHandler,
	}
}

type handlerManager struct {
	errorHandler ErrorHandler
	first        HandlerChain
	last         HandlerChain
}

func (h *handlerManager) SetErrorHandler(handler ErrorHandler) {
	if handler == nil {
		return
	}

	h.errorHandler = handler
}

func (h *handlerManager) Use(hfHandler HttpFilterHandler) HandlerManager {
	if hfHandler == nil || hfHandler.Disable() {
		return h
	}

	handler := NewHandlerChain(hfHandler)

	if h.first == nil {
		h.first = handler
		h.last = handler
		return h
	}

	h.last.SetNext(handler)
	h.last = handler
	return h
}

func (h *handlerManager) handle(c Context, phase ActionPhase) (status api.StatusType) {
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
		h.first = NewHandlerChain(PassthroughHttpFilterHandler{})
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
