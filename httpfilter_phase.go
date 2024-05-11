package gonvoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// HttpFilterPhaseDirector is responsible for managing the Http Filter processor across different phases.
// It oversees two distinct phases:
// Decode, which handles HTTP Request flows, and
// Encode, which manages HTTP Response flows.
type HttpFilterPhaseDirector struct {
	decode HttpFilterDecodeProcessor
	encode HttpFilterEncodeProcessor
}

type HttpFilterPhaseFunc func(Context, HttpFilterPhaseDirector) (HttpFilterAction, error)

func NewHttpFilterPhaseDirector(decode, encode HttpFilterProcessor) HttpFilterPhaseDirector {
	return HttpFilterPhaseDirector{
		decode: decode,
		encode: encode,
	}
}

func newDecodeHeadersPhase(header api.RequestHeaderMap) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		c.SetRequestHeader(header)

		if err := d.decode.HandleOnRequestHeader(c); err != nil {
			return ActionContinue, err
		}

		if c.IsRequestBodyReadable() {
			c.RequestHeader().Del(HeaderXContentOperation)
			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func newDecodeDataPhase(buffer api.BufferInstance, endStream bool) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		isRequestBodyAccessible := c.IsRequestBodyReadable() || c.IsRequestBodyWriteable()
		if !isRequestBodyAccessible {
			return ActionSkip, nil
		}

		if buffer.Len() > 0 {
			c.SetRequestBody(buffer)
		}

		if endStream {
			return ActionContinue, d.decode.HandleOnRequestBody(c)
		}

		return ActionPause, nil
	}
}

func newEncodeHeadersPhase(header api.ResponseHeaderMap) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		c.SetResponseHeader(header)

		if err := d.encode.HandleOnResponseHeader(c); err != nil {
			return ActionContinue, err
		}

		if c.IsResponseBodyReadable() {
			c.ResponseHeader().Del(HeaderXContentOperation)
			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func newEncodeDataPhase(buffer api.BufferInstance, endStream bool) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		isResponseBodyAccessible := c.IsResponseBodyReadable() || c.IsResponseBodyWriteable()
		if !isResponseBodyAccessible {
			return ActionSkip, nil
		}

		if buffer.Len() > 0 {
			c.SetResponseBody(buffer)
		}

		if endStream {
			return ActionContinue, d.encode.HandleOnResponseBody(c)
		}

		return ActionPause, nil
	}
}
