package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterAction uint

const (
	ActionContinue HttpFilterAction = iota + 1
	ActionPause
)

type HttpFilterHandlerManager interface {
	SetErrorHandler(ErrorHandler)
	RegisterHandler(HttpFilterHandler)
	Serve(c Context, ctrl HttpFilterPhaseController) api.StatusType
}

type DefaultHttpFilterHandlerManager struct {
	errorHandler ErrorHandler
	entrypoint   httpFilterProcessor
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
	if h.entrypoint == nil {
		h.entrypoint = processor
		h.last = processor
		return
	}

	h.last.SetNext(processor)
	h.last = processor
}

func (h *DefaultHttpFilterHandlerManager) Serve(c Context, ctrl HttpFilterPhaseController) (status api.StatusType) {
	var (
		action HttpFilterAction
		err    error
	)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w; %v", errs.ErrPanic, r)
		}

		if err != nil {
			h.errorHandler(c, err)
		}

		status = c.StatusType()

		switch action {
		case ActionPause:
			status = api.StopAndBuffer
		case ActionContinue:
			fallthrough
		default:
			status = c.StatusType()
		}
	}()

	if h.entrypoint == nil {
		h.entrypoint = NewHttpFilterProcessor(PassthroughHttpFilterHandler{})
	}

	action, err = ctrl.Handle(c, h.entrypoint)
	return
}
