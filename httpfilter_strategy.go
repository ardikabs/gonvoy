package gonvoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// HttpFilterPhaseStrategy is an interface representing the Sstrategy for handling different Envoy's filter phases.
// Such as DecodeHeaders, DecodeData, EncodeHeaders and EncodeData
type HttpFilterPhaseStrategy interface {

	// Execute executes the strategy for the specified HTTP filter phase.
	// Keep in mind that the 'first' HttpFilterProcessor is intended for managing HTTP Request flows,
	// while the 'last' HttpFilterProcessor is used for handling HTTP Response flows.
	// In other words, use the 'first' HttpFilterProcessor during Decode phases,
	// and utilize the 'last' HttpFilterProcessor during Encode phases.
	//
	Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error)
}

// --------------------------------------------------------------------------------------------

// newDecodeHeadersStrategy returns a strategy used for DecodeHeaders phase
func newDecodeHeadersStrategy(header api.RequestHeaderMap) HttpFilterPhaseStrategy {
	return &decodeHeadersStrategy{
		header: header,
	}
}

type decodeHeadersStrategy struct {
	header api.RequestHeaderMap
}

func (p *decodeHeadersStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	c.SetRequestHeader(p.header)

	if err := first.HandleOnRequestHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsRequestBodyReadable() {
		c.RequestHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// --------------------------------------------------------------------------------------------

// newDecodeDataStrategy returns a strategy used for DecodeData phase
func newDecodeDataStrategy(buffer api.BufferInstance, endStream bool) HttpFilterPhaseStrategy {
	return &decodeDataStrategy{
		buffer:    buffer,
		endStream: endStream,
	}
}

type decodeDataStrategy struct {
	buffer    api.BufferInstance
	endStream bool
}

func (p *decodeDataStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	isRequestBodyAccessible := c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
	if !isRequestBodyAccessible {
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

// --------------------------------------------------------------------------------------------

// newEncodeHeadersStrategy returns a strategy used for EncodeHeaders phase.
func newEncodeHeadersStrategy(header api.ResponseHeaderMap) HttpFilterPhaseStrategy {
	return &encodeHeadersStrategy{
		header: header,
	}
}

type encodeHeadersStrategy struct {
	header api.ResponseHeaderMap
}

func (p *encodeHeadersStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	c.SetResponseHeader(p.header)

	if err := last.HandleOnResponseHeader(c); err != nil {
		return ActionContinue, err
	}

	if c.IsResponseBodyReadable() {
		c.ResponseHeader().Del(HeaderXContentOperation)
		return ActionPause, nil
	}

	return ActionContinue, nil
}

// --------------------------------------------------------------------------------------------

// newEncodeDataStrategy returns a strategy used for EncodeData phase.
func newEncodeDataStrategy(buffer api.BufferInstance, endStream bool) HttpFilterPhaseStrategy {
	return &encodeDataStrategy{
		buffer:    buffer,
		endStream: endStream,
	}
}

type encodeDataStrategy struct {
	buffer    api.BufferInstance
	endStream bool
}

func (p *encodeDataStrategy) Execute(c Context, first, last HttpFilterProcessor) (HttpFilterAction, error) {
	isResponseBodyAccessible := c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
	if !isResponseBodyAccessible {
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
