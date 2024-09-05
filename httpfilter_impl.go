package gaetway

import (
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var _ api.StreamFilter = &httpFilterImpl{}

// httpFilterImpl is an HTTP Filter implementation for Envoy.
type httpFilterImpl struct {
	srv HttpFilterServer
}

func (f *httpFilterImpl) OnLog() { f.srv.Complete() }

func (f *httpFilterImpl) OnDestroy(reason api.DestroyReason) { f.srv = nil }

func (f *httpFilterImpl) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	result := f.srv.ServeDecodeFilter(f.handleRequestHeader(header))
	return result.Status
}

func (f *httpFilterImpl) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	result := f.srv.ServeDecodeFilter(f.handleRequestBody(buffer, endStream))
	return result.Status
}

func (f *httpFilterImpl) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	result := f.srv.ServeEncodeFilter(f.handleResponseHeader(header))
	return result.Status
}

func (f *httpFilterImpl) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	result := f.srv.ServeEncodeFilter(f.handleResponseBody(buffer, endStream))
	return result.Status
}

func (f *httpFilterImpl) handleRequestHeader(header api.RequestHeaderMap) HttpFilterDecoderFunc {
	return func(c Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		c.LoadRequestHeaders(header)

		if err := p.HandleOnRequestHeader(c); err != nil {
			return ActionContinue, err
		}

		if c.IsRequestBodyWritable() {
			// If content length is omitted, there's no need for the filter manager to buffer the request headers.
			// Therefore, we can continue the flow.
			if shouldOmitContentLengthOnRequest(c, header) {
				return ActionContinue, nil
			}

			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func (f *httpFilterImpl) handleRequestBody(buffer api.BufferInstance, endStream bool) HttpFilterDecoderFunc {
	return func(c Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		if !c.IsRequestBodyAccessible() {
			return ActionSkip, nil
		}

		c.LoadRequestBody(buffer, endStream)

		if !endStream {
			// Wait -- we'll be called again when the complete body is buffered
			// at the Envoy host side.
			return ActionWait, nil
		}

		return ActionContinue, p.HandleOnRequestBody(c)
	}
}

func (f *httpFilterImpl) handleResponseHeader(header api.ResponseHeaderMap) HttpFilterEncoderFunc {
	return func(c Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		c.LoadResponseHeaders(header)

		if err := p.HandleOnResponseHeader(c); err != nil {
			return ActionContinue, err
		}

		// During the Encode phases or HTTP Response flows,
		// if a user needs access to the HTTP Response Body, whether for reading or writing,
		// the EncodeHeaders phase should return with ActionPause (StopAndBuffer status) action.
		// This is necessary because the Response Header must be buffered in Envoy's filter-manager.
		// If this buffering is not done, the Response Header might be sent to the downstream client prematurely,
		// preventing the filter from returning a custom error response in case of unexpected events during processing.
		//
		// Hence, we opted for the IsResponseBodyReadable check instead of IsResponseBodyWritable.
		// It's worth noting that the behavior differs in the Decode phase because the stream flow is directed towards the upstream.
		// This means that even if DecodeHeaders has returned with ActionContinue (Continue status),
		// DecodeData is still under supervision within Envoy's filter-manager state.
		if c.IsResponseBodyReadable() {
			// Regardless of whether the content length is omitted, the filter manager needs to buffer the response headers.
			// This is to safeguard against unforeseen events during processing, allowing us to interrupt it with a custom error response.
			if c.IsResponseBodyWritable() {
				// Ignore the return value of shouldOmitContentLengthOnResponse.
				_ = shouldOmitContentLengthOnResponse(c, header)
			}

			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

// Attention! Please be mindful of the Listener or Cluster per_connection_buffer_limit_bytes value
// when enabling the response body access on ConfigOptions (EnableResponseBodyRead or EnableResponseBodyWrite).
// The default value set by Envoy is 1MB. If the response body size exceeds this limit, the process will be halted.
// TODO(ardikabs): Upgrade to recent version where the issue is resolved, ref(https://github.com/envoyproxy/envoy/pull/34240).
func (f *httpFilterImpl) handleResponseBody(buffer api.BufferInstance, endStream bool) HttpFilterEncoderFunc {
	return func(c Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		if !c.IsResponseBodyAccessible() {
			return ActionSkip, nil
		}

		c.LoadResponseBody(buffer, endStream)

		if !endStream {
			// Wait -- we'll be called again when the complete body is buffered
			// at the Envoy host side.
			return ActionWait, nil
		}

		return ActionContinue, p.HandleOnResponseBody(c)
	}
}

func (*httpFilterImpl) DecodeTrailers(api.RequestTrailerMap) api.StatusType  { return api.Continue }
func (*httpFilterImpl) EncodeTrailers(api.ResponseTrailerMap) api.StatusType { return api.Continue }
func (*httpFilterImpl) OnLogDownstreamPeriodic()                             {}
func (*httpFilterImpl) OnLogDownstreamStart()                                {}
