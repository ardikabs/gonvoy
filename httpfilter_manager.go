package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterAction uint

const (
	ActionSkip HttpFilterAction = iota
	ActionContinue
	ActionPause
)

type HttpFilterManager interface {
	// IsFilterPhaseDisabled specifies whether given http filter phase is disabled or not
	//
	IsFilterPhaseDisabled(HttpFilterPhase) bool

	// SetErrorHandler sets a custom error handler for an Http Filter
	//
	SetErrorHandler(ErrorHandler)

	// RegisterHTTPFilterHandler adds an Http Filter Handler to the chain,
	// which should be run during filter startup (HttpFilter.OnBegin).
	// It's important to note the order when registering filter handlers.
	// While HTTP requests follow FIFO sequences, HTTP responses follow LIFO sequences.
	//
	// Example usage:
	//	func (f *UserFilter) OnBegin(c RuntimeContext) error {
	//		...
	//		c.RegisterHTTPFilterHandler(handlerA)
	//		c.RegisterHTTPFilterHandler(handlerB)
	//		c.RegisterHTTPFilterHandler(handlerC)
	//		c.RegisterHTTPFilterHandler(handlerD)
	//	}
	//
	// During HTTP requests, traffic flows from `handlerA -> handlerB -> handlerC -> handlerD`.
	// During HTTP responses, traffic flows in reverse: `handlerD -> handlerC -> handlerB -> handlerA`.
	//
	RegisterHTTPFilterHandler(HttpFilterHandler)

	// ServeHTTPFilter serves the Http Filter for the specified phase.
	// This method is designed for internal use as it is directly invoked within each filter instance's phases.
	// Available phases: OnRequestHeaderPhase, OnRequestBodyPhase, OnResponseHeaderPhase, OnResponseBodyPhase.
	//
	ServeHTTPFilter(strategy HttpFilterPhaseStrategy) api.StatusType
}

func newHttpFilterManager(ctx Context, cfg *globalConfig) *httpFilterManager {
	return &httpFilterManager{
		ctx:           ctx,
		disabledPhase: cfg.disabledHttpFilterPhases,
		errorHandler:  DefaultErrorHandler,
	}
}

type httpFilterManager struct {
	ctx           Context
	disabledPhase []HttpFilterPhase

	errorHandler ErrorHandler
	first        HttpFilterProcessor
	last         HttpFilterProcessor
}

func (h *httpFilterManager) IsFilterPhaseDisabled(phase HttpFilterPhase) bool {
	return util.In(phase, h.disabledPhase...)
}

func (h *httpFilterManager) SetErrorHandler(handler ErrorHandler) {
	if util.IsNil(handler) {
		return
	}

	h.errorHandler = handler
}

func (h *httpFilterManager) RegisterHTTPFilterHandler(handler HttpFilterHandler) {
	if util.IsNil(handler) || handler.Disable() {
		return
	}

	proc := newHttpFilterProcessor(handler)
	if h.first == nil {
		h.first = proc
		h.last = proc
		return
	}

	proc.SetPrevious(h.last)
	h.last.SetNext(proc)
	h.last = proc
}

func (h *httpFilterManager) ServeHTTPFilter(strategy HttpFilterPhaseStrategy) (status api.StatusType) {
	var (
		action HttpFilterAction
		err    error
	)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w; %v", errs.ErrPanic, r)
		}

		if err != nil {
			status = h.errorHandler(h.ctx, err)
			return
		}

		switch action {
		case ActionContinue:
			status = h.ctx.StatusType()
		case ActionPause:
			status = api.StopAndBuffer
		default:
			status = api.Continue
		}
	}()

	if h.first == nil || h.last == nil {
		return
	}

	action, err = strategy.Execute(h.ctx, h.first, h.last)
	return
}
