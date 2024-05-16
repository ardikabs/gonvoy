package gonvoy

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

// HttpFilterPhaseDirector is responsible for managing the Http Filter processor during different phases.
// It oversees two main phases:
// - Decode: Handles HTTP Request flows.
// - Encode: Manages HTTP Response flows.
type HttpFilterPhaseDirector struct {
	decode HttpFilterDecodeProcessor
	encode HttpFilterEncodeProcessor
}

// HttpFilterPhaseFunc is a function type that is used during each phase of an HTTP filter.
// It takes a `Context` and a `HttpFilterPhaseDirector` as parameters and returns an `HttpFilterAction` and an error.
type HttpFilterPhaseFunc func(Context, HttpFilterPhaseDirector) (HttpFilterAction, error)

func newHttpFilterPhaseDirector(decode, encode HttpFilterProcessor) HttpFilterPhaseDirector {
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

		if c.IsRequestBodyWriteable() {
			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func newDecodeDataPhase(buffer api.BufferInstance, endStream bool) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		if !isRequestBodyAccessible(c) {
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

		// During the Encode phases or HTTP Response flows,
		// if a user needs access to the HTTP Response Body, whether for reading or writing,
		// the EncodeHeaders phase should return with ActionPause (StopAndBuffer status) status.
		// This is necessary because the Response Header must be buffered in Envoy's filter-manager.
		// If this buffering is not done, the Response Header might be sent to the downstream client prematurely,
		// preventing the filter from returning a custom error response in case of unexpected events during processing.
		//
		// Hence, we opted for the IsResponseBodyReadable check instead of IsResponseBodyWritable.
		// It's worth noting that the behavior differs in the Decode phase because the stream flow is directed towards the upstream.
		// This means that even if DecodeHeaders has returned with ActionContinue (Continue status),
		// DecodeData is still under supervision within Envoy's filter-manager state.
		if c.IsResponseBodyReadable() {
			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func newEncodeDataPhase(buffer api.BufferInstance, endStream bool) HttpFilterPhaseFunc {
	return func(c Context, d HttpFilterPhaseDirector) (HttpFilterAction, error) {
		// Attention! Please be mindful of the Listener or Cluster per_connection_buffer_limit_bytes value
		// when enabling the response body access on ConfigOptions (EnableResponseBodyRead or EnableResponseBodyWrite).
		// The default value set by Envoy is 1MB. If the response body size exceeds this limit, the process will be halted.
		// Although it's unclear whether this is considered a bug or a limitation at present,
		// the Envoy Golang HTTP Filter library currently returns a 413 status code with a PayloadTooLarge message in such cases.
		// Code references: https://github.com/envoyproxy/envoy/blob/v1.29.4/contrib/golang/filters/http/source/processor_state.cc#L362-L371.
		//
		if !isResponseBodyAccessible(c) {
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
