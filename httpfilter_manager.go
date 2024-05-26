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

	// ActionWait is an operation action that waits the current filter phase.
	// The purpose of ActionWait is specifically for data streaming phases such as DecodeData or EncodeData,
	// where the filter phase needs to wait until the entire body is buffered on the Envoy host side.
	// From Envoy's perspective, this is equivalent to a StopNoBuffer status.
	ActionWait
)

// HttpFilterDecoderFunc is a function type that represents a decoder for HTTP filters.
// It is used during the decoder phases, which is when the filter processes the incoming request.
type HttpFilterDecoderFunc func(Context, HttpFilterDecodeProcessor) (HttpFilterAction, error)

// HttpFilterEncoderFunc is a function type that represents an encoder for HTTP filters.
// It is used during the encoder phases, which is when the filter processes the upstream response.
type HttpFilterEncoderFunc func(Context, HttpFilterEncodeProcessor) (HttpFilterAction, error)

// HttpFilterController represents a controller for managing HTTP filters.
type HttpFilterController interface {
	// SetErrorHandler sets a custom error handler for an HTTP Filter.
	// The error handler will be called when an error occurs during the execution of the HTTP filter.
	//
	SetErrorHandler(handler ErrorHandler)

	// AddHandler adds an HTTP Filter Handler to the controller,
	// which should be run during filter startup (HttpFilter.OnBegin).
	// It's important to note the order when adding filter handlers.
	// While HTTP requests follow FIFO sequences, HTTP responses follow LIFO sequences.
	//
	// Example usage:
	//
	//	func (f *UserFilter) OnBegin(c RuntimeContext, ctrl HttpFilterController) error {
	//		...
	//		ctrl.AddHandler(handlerA)
	//		ctrl.AddHandler(handlerB)
	//		ctrl.AddHandler(handlerC)
	//		ctrl.AddHandler(handlerD)
	//	}
	//
	// During HTTP requests, traffic flows from `handlerA -> handlerB -> handlerC -> handlerD`.
	// During HTTP responses, traffic flows in reverse: `handlerD -> handlerC -> handlerB -> handlerA`.
	//
	AddHandler(handler HttpFilterHandler)
}

// HttpFilterServer is an HTTP filter server used for handling HTTP requests and responses.
// It provides methods to serve decode and encode filters, as well as to finalize the processing of the HTTP filter server.
type HttpFilterServer interface {
	// ServeDecodeFilter serves decode phase of an HTTP filter.
	// This method is designed for internal use as it is used during the decoding phase only.
	// Decode phase is when the filter processes the incoming request, which consist of processing headers and body.
	//
	ServeDecodeFilter(HttpFilterDecoderFunc) *HttpFilterResult

	// ServeEncodeFilter serves encode phase of an HTTP filter.
	// This method is designed for internal use as it is used during the encoding phase only.
	// Encode phase is when the filter processes the upstream response, which consist of processing headers and body.
	//
	ServeEncodeFilter(HttpFilterEncoderFunc) *HttpFilterResult

	// Finalize is called when the HTTP filter server has finalized processing.
	//
	Finalize()
}

// HttpFilterCompletionFunc represents a function type for completing an HTTP filter.
type HttpFilterCompletionFunc func()

func newHttpFilterManager(c Context) *httpFilterManager {
	return &httpFilterManager{
		ctx:          c,
		errorHandler: DefaultErrorHandler,
	}
}

// httpFilterManager represents an HTTP filter manager used for managing HTTP filters.
// It implements the HttpFilterController and HttpFilterServer interfaces.
type httpFilterManager struct {
	ctx          Context
	errorHandler ErrorHandler
	first        HttpFilterProcessor
	last         HttpFilterProcessor
	completer    HttpFilterCompletionFunc
}

func (m *httpFilterManager) SetErrorHandler(handler ErrorHandler) {
	if util.IsNil(handler) {
		return
	}

	m.errorHandler = handler
}

func (m *httpFilterManager) AddHandler(handler HttpFilterHandler) {
	if util.IsNil(handler) || handler.Disable() {
		return
	}

	proc := newHttpFilterProcessor(handler)
	if m.first == nil {
		m.first = proc
		m.last = proc
		return
	}

	proc.SetPrevious(m.last)
	m.last.SetNext(proc)
	m.last = proc
}

func (m *httpFilterManager) ServeDecodeFilter(fn HttpFilterDecoderFunc) (res *HttpFilterResult) {
	res = initHttpFilterResult()
	defer res.Finalize(m.ctx, m.errorHandler)
	if m.first == nil {
		return
	}

	res.Action, res.Err = fn(m.ctx, m.first)
	return
}

func (m *httpFilterManager) ServeEncodeFilter(fn HttpFilterEncoderFunc) (res *HttpFilterResult) {
	res = initHttpFilterResult()
	defer res.Finalize(m.ctx, m.errorHandler)
	if m.last == nil {
		return
	}

	res.Action, res.Err = fn(m.ctx, m.last)
	return
}

func (m *httpFilterManager) Finalize() {
	if m.completer != nil {
		m.completer()
	}
}

func initHttpFilterResult() *HttpFilterResult {
	return &HttpFilterResult{
		Action: ActionSkip,
		Status: api.Continue,
	}
}

// HttpFilterResult represents the result of an HTTP filter operation.
type HttpFilterResult struct {
	// Action represents the action to be taken based on the filter result.
	Action HttpFilterAction
	// Err represents the error encountered during the filter operation, if any.
	Err error
	// Status represents the final status of the filter processing.
	Status api.StatusType
}

// Finalize performs the finalization logic for the HttpFilterResult.
// It recovers from any panics, sets the appropriate error and status,
// and determines the action to be taken based on the result.
func (res *HttpFilterResult) Finalize(c Context, errorHandler ErrorHandler) {
	if r := recover(); r != nil {
		res.Err = fmt.Errorf("%w; %v", errs.ErrPanic, r)
	}

	if res.Err != nil {
		res.Status = errorHandler(c, res.Err)
		return
	}

	switch res.Action {
	case ActionContinue:
		res.Status = c.StatusType()
	case ActionPause:
		res.Status = api.StopAndBuffer
	case ActionWait:
		res.Status = api.StopNoBuffer
	default:
		res.Status = api.Continue
	}
}
