package gonvoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type HttpFilterPhase uint

const (
	OnRequestHeaderPhase HttpFilterPhase = iota
	OnResponseHeaderPhase
	OnRequestBodyPhase
	OnResponseBodyPhase
)

// HttpFilterPhaseStrategy is an interface representing the Sstrategy for handling a phase of an HTTP filter.
type HttpFilterPhaseStrategy interface {

	// Execute executes the strategy for the specified HTTP filter phase.
	// Keep in mind that the 'first' HttpFilterProcessor is intended for managing HTTP Request flows,
	// while the 'last' HttpFilterProcessor is used for handling HTTP Response flows.
	// In other words, use the 'first' HttpFilterProcessor during OnRequestHeader and OnRequestBody phases,
	// and utilize the 'last' HttpFilterProcessor during OnResponseHeader and OnResponseBody phases.
	//
	Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error)
}

// newRequestHeaderStrategy returns an HttpFilterPhaseStrategy based on OnRequestHeaderPhase phase
func newRequestHeaderStrategy(header api.RequestHeaderMap) HttpFilterPhaseStrategy {
	return &requestHeaderStrategy{
		phase:  OnRequestHeaderPhase,
		header: header,
	}
}

type requestHeaderStrategy struct {
	phase  HttpFilterPhase
	header api.RequestHeaderMap
}

func (p *requestHeaderStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	if c.IsFilterPhaseDisabled(p.phase) {
		return ActionSkip, nil
	}

	c.SetRequestHeader(p.header)

	if err := first.HandleOnRequestHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsRequestBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// newRequestBodyStrategy returns an HttpFilterPhaseStrategy based on OnRequestBodyPhase phase
func newRequestBodyStrategy(buffer api.BufferInstance, endStream bool) HttpFilterPhaseStrategy {
	return &requestBodyStrategy{
		phase:     OnRequestBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type requestBodyStrategy struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *requestBodyStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
	if c.IsFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionSkip, nil
	}

	if p.buffer.Len() > 0 {
		c.SetRequestBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, first.HandleOnRequestBody(c)
	}

	return ActionPause, nil
}

// newResponseHeaderStrategy returns an HttpFilterPhaseStrategy based on OnResponseHeaderPhase phase
func newResponseHeaderStrategy(header api.ResponseHeaderMap) HttpFilterPhaseStrategy {
	return &responseHeaderStrategy{
		phase:  OnResponseHeaderPhase,
		header: header,
	}
}

type responseHeaderStrategy struct {
	phase  HttpFilterPhase
	header api.ResponseHeaderMap
}

func (p *responseHeaderStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	if c.IsFilterPhaseDisabled(p.phase) {
		return ActionSkip, nil
	}

	c.SetResponseHeader(p.header)

	if err := last.HandleOnResponseHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsResponseBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// newResponseBodyStrategy returns an HttpFilterPhaseStrategy based on OnResponseBodyPhase phase
func newResponseBodyStrategy(buffer api.BufferInstance, endStream bool) HttpFilterPhaseStrategy {
	return &responseBodyStrategy{
		phase:     OnResponseBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type responseBodyStrategy struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *responseBodyStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
	if c.IsFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionSkip, nil
	}

	if p.buffer.Len() > 0 {
		c.SetResponseBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, last.HandleOnResponseBody(c)
	}

	return ActionPause, nil
}
