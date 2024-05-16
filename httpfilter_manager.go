package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/errs"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// HttpFilterAction represents the action to be taken on each phase of an HTTP filter.
type HttpFilterAction uint

const (
	// ActionSkip is basically a no-op action, which mean the filter phase will be skipped.
	// From Envoy perspective, this is equivalent to a Continue status.
	ActionSkip HttpFilterAction = iota
	// ActionContinue is operation action that continues the current filter phase.
	// From Envoy perspective, this is equivalent to a Continue status.
	// Although the status is identical to ActionSkip, the purpose of ActionContinue
	// is to indicate that the current phase is operating correctly and needs to advance to the subsequent filter phase.
	ActionContinue
	// ActionPause is an operation action that pause the current filter phase.
	// This pause could be essential as future filter phases may rely on the processes of preceding phases.
	// For example, the EncodeData phase may require header alterations, which are established in the EncodeHeaders phase.
	// Therefore, EncodeHeaders should return with ActionPause, and the header changes may occur later in the EncodeData phase.
	// From Envoy perspective, this is equivalent to a StopAndBuffer status.
	// Be aware, when employing this action, the subsequent filter phase must return with ActionContinue,
	// otherwise the filter chain will be hanging.
	ActionPause
)

// HttpFilterManager represents an interface for managing HTTP filters.
type HttpFilterManager interface {
	// SetErrorHandler sets a custom error handler for an HTTP Filter.
	SetErrorHandler(ErrorHandler)

	// RegisterHTTPFilterHandler adds an HTTP Filter Handler to the chain,
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
	RegisterHTTPFilterHandler(HttpFilterHandler)

	// ServeHTTPFilter serves the HTTP Filter for the specified phase.
	// This method is designed for internal use as it is directly invoked within each filter instance's phases.
	// Refers to HttpFilterPhaseFunc.
	ServeHTTPFilter(HttpFilterPhaseFunc) api.StatusType
}

func newHttpFilterManager(ctx Context) *httpFilterManager {
	return &httpFilterManager{
		ctx:          ctx,
		errorHandler: DefaultErrorHandler,
	}
}

type httpFilterManager struct {
	ctx          Context
	errorHandler ErrorHandler
	first        HttpFilterProcessor
	last         HttpFilterProcessor
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

func (h *httpFilterManager) ServeHTTPFilter(phase HttpFilterPhaseFunc) (status api.StatusType) {
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

	director := newHttpFilterPhaseDirector(h.first, h.last)
	action, err = phase(h.ctx, director)
	return
}
