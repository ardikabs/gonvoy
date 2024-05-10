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

// HttpFilterPhaseController is an interface that represents the controller for a phase of an HTTP filter.
type HttpFilterPhaseController interface {
	Handle(c Context, proc HttpFilterProcessor) (HttpFilterAction, error)
}

// newRequestHeaderController returns an Http Filter Phase controller for OnRequestHeaderPhase phase
func newRequestHeaderController(header api.RequestHeaderMap) HttpFilterPhaseController {
	return &requestHeaderController{
		phase:  OnRequestHeaderPhase,
		header: header,
	}
}

type requestHeaderController struct {
	phase  HttpFilterPhase
	header api.RequestHeaderMap
}

func (p *requestHeaderController) Handle(c Context, proc HttpFilterProcessor) (HttpFilterAction, error) {
	if c.IsFilterPhaseDisabled(p.phase) {
		return ActionSkip, nil
	}

	c.SetRequestHeader(p.header)

	if err := proc.HandleOnRequestHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsRequestBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// newResponseHeaderController returns an Http Filter Phase controller for OnResponseHeaderPhase phase
func newResponseHeaderController(header api.ResponseHeaderMap) HttpFilterPhaseController {
	return &responseHeaderController{
		phase:  OnResponseHeaderPhase,
		header: header,
	}
}

type responseHeaderController struct {
	phase  HttpFilterPhase
	header api.ResponseHeaderMap
}

func (p *responseHeaderController) Handle(c Context, proc HttpFilterProcessor) (HttpFilterAction, error) {
	if c.IsFilterPhaseDisabled(p.phase) {
		return ActionSkip, nil
	}

	c.SetResponseHeader(p.header)

	if err := proc.HandleOnResponseHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsResponseBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// newRequestBodyController returns an Http Filter Phase controller for OnRequestBodyPhase phase
func newRequestBodyController(buffer api.BufferInstance, endStream bool) HttpFilterPhaseController {
	return &requestBodyController{
		phase:     OnRequestBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type requestBodyController struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *requestBodyController) Handle(c Context, proc HttpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
	if c.IsFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionSkip, nil
	}

	if p.buffer.Len() > 0 {
		c.SetRequestBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, proc.HandleOnRequestBody(c)
	}

	return ActionPause, nil
}

// newResponseBodyController returns an Http Filter Phase controller for OnResponseBodyPhase phase
func newResponseBodyController(buffer api.BufferInstance, endStream bool) HttpFilterPhaseController {
	return &responseBodyController{
		phase:     OnResponseBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type responseBodyController struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *responseBodyController) Handle(c Context, proc HttpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
	if c.IsFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionSkip, nil
	}

	if p.buffer.Len() > 0 {
		c.SetResponseBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, proc.HandleOnResponseBody(c)
	}

	return ActionPause, nil
}
