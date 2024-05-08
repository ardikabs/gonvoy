package gonvoy

import "github.com/envoyproxy/envoy/contrib/golang/common/go/api"

type HttpFilterPhase uint

const (
	OnRequestHeaderPhase HttpFilterPhase = iota
	OnResponseHeaderPhase
	OnRequestBodyPhase
	OnResponseBodyPhase
)

type HttpFilterPhaseController interface {
	Handle(c Context, proc httpFilterProcessor) (HttpFilterAction, error)
}

// NewRequestHeaderPhaseCtrl returns an Http Filter Phase controller for OnRequestHeaderPhase phase
func NewRequestHeaderPhaseCtrl(header api.RequestHeaderMap) HttpFilterPhaseController {
	return &requestHeaderPhaseController{
		phase:  OnRequestHeaderPhase,
		header: header,
	}
}

type requestHeaderPhaseController struct {
	phase  HttpFilterPhase
	header api.RequestHeaderMap
}

func (p *requestHeaderPhaseController) Handle(c Context, proc httpFilterProcessor) (HttpFilterAction, error) {
	if c.IsHttpFilterPhaseDisabled(p.phase) {
		return ActionContinue, nil
	}

	c.SetRequestHeader(p.header)

	if c.IsRequestBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, proc.HandleOnRequestHeader(c)
}

// NewResponseHeaderPhaseCtrl returns an Http Filter Phase controller for OnResponseHeaderPhase phase
func NewResponseHeaderPhaseCtrl(header api.ResponseHeaderMap) HttpFilterPhaseController {
	return &responseHeaderPhaseController{
		phase:  OnResponseHeaderPhase,
		header: header,
	}
}

type responseHeaderPhaseController struct {
	phase  HttpFilterPhase
	header api.ResponseHeaderMap
}

func (p *responseHeaderPhaseController) Handle(c Context, proc httpFilterProcessor) (HttpFilterAction, error) {
	if c.IsHttpFilterPhaseDisabled(p.phase) {
		return ActionContinue, nil
	}

	c.SetResponseHeader(p.header)

	if c.IsResponseBodyWriteable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, proc.HandleOnResponseHeader(c)
}

// NewRequestBodyPhaseCtrl returns an Http Filter Phase controller for OnRequestBodyPhase phase
func NewRequestBodyPhaseCtrl(buffer api.BufferInstance, endStream bool) HttpFilterPhaseController {
	return &requestBodyPhaseController{
		phase:     OnRequestBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type requestBodyPhaseController struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *requestBodyPhaseController) Handle(c Context, proc httpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
	if c.IsHttpFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionContinue, nil
	}

	if p.buffer.Len() > 0 {
		c.SetRequestBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, proc.HandleOnRequestBody(c)
	}

	return ActionPause, nil
}

// NewResponseBodyPhaseCtrl returns an Http Filter Phase controller for OnResponseBodyPhase phase
func NewResponseBodyPhaseCtrl(buffer api.BufferInstance, endStream bool) HttpFilterPhaseController {
	return &responseBodyPhaseController{
		phase:     OnResponseBodyPhase,
		buffer:    buffer,
		endStream: endStream,
	}
}

type responseBodyPhaseController struct {
	phase     HttpFilterPhase
	buffer    api.BufferInstance
	endStream bool
}

func (p *responseBodyPhaseController) Handle(c Context, proc httpFilterProcessor) (HttpFilterAction, error) {
	isBodyAccessible := c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
	if c.IsHttpFilterPhaseDisabled(p.phase) || !isBodyAccessible {
		return ActionContinue, nil
	}

	if p.buffer.Len() > 0 {
		c.SetResponseBody(p.buffer)
	}

	if p.endStream {
		return ActionContinue, proc.HandleOnResponseBody(c)
	}

	return ActionPause, nil
}
