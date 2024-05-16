package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoyhttp "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

// RunHttpFilter is an entrypoint for onboarding User's HTTP filters at runtime.
// It must be declared inside `func init()` blocks in main package.
// Example usage:
//
//	package main
//	func init() {
//		RunHttpFilter(new(UserHttpFilterInstance), ConfigOptions{
//			BaseConfig: new(UserHttpFilterConfig),
//		})
//	}
func RunHttpFilter(filter HttpFilter, options ConfigOptions) {
	envoyhttp.RegisterHttpFilterConfigFactoryAndParser(
		filter.Name(),
		httpFilterFactory(filter),
		newConfigParser(options),
	)
}

// HttpFilter defines an interface for an HTTP filter used in Envoy.
// It provides methods for managing filter names, startup, and completion.
// This interface is specifically designed as a mechanism for onboarding the user HTTP filters to Envoy.
// HttpFilter is always renewed for every request.
type HttpFilter interface {
	// Name returns the name of the filter used in Envoy.
	//
	// The Name method should return a unique name for the filter.
	// This name is used to identify the filter in the Envoy configuration.
	Name() string

	// OnBegin is executed during filter startup.
	//
	// The OnBegin method is called when the filter is initialized.
	// It can be used by the user to perform filter preparation tasks, such as:
	// - Retrieving filter configuration (if provided)
	// - Registering filter handlers
	// - Capturing user-generated metrics
	//
	// If an error is returned, the filter will be ignored.
	OnBegin(c RuntimeContext) error

	// OnComplete is executed when the filter is completed.
	//
	// The OnComplete method is called when the filter is about to be destroyed.
	// It can be used by the user to perform filter completion tasks, such as:
	// - Capturing user-generated metrics
	// - Cleaning up resources
	//
	// If an error is returned, nothing happens.
	OnComplete(c RuntimeContext) error
}

func httpFilterFactory(filter HttpFilter) api.StreamFilterConfigFactory {
	if util.IsNil(filter) {
		panic("httpFilterFactory: httpFilter shouldn't be a nil")
	}

	return func(cfg interface{}) api.StreamFilterFactory {
		config, ok := cfg.(*globalConfig)
		if !ok {
			panic(fmt.Sprintf("httpFilterFactory: unexpected config type '%T', expecting '%T'", cfg, config))
		}

		return func(cb api.FilterCallbackHandler) api.StreamFilter {
			log := newLogger(cb)
			opts := []ContextOption{WithContextConfig(config), WithContextLogger(log)}
			ctx, err := newContext(cb, opts...)
			if err != nil {
				log.Error(err, "failed to initialize filter, ignoring filter ...")
				return NoOpHttpFilter
			}

			complete, err := renewHttpFilter(ctx, filter)
			if err != nil {
				log.Error(err, "failed to start filter, ignoring filter ...")
				return NoOpHttpFilter
			}

			server, err := getHttpFilterServer(ctx)
			if err != nil {
				log.Error(err, "failed to get filter server, ignoring filter ...")
				return NoOpHttpFilter
			}

			return &httpFilterInstance{
				srv:      server,
				complete: complete,
			}
		}
	}
}

var _ api.StreamFilter = &httpFilterInstance{}

type httpFilterInstance struct {
	api.PassThroughStreamFilter

	srv      HttpFilterServer
	complete CompleteFunc
}

func (f *httpFilterInstance) OnLog() {
	if f.complete != nil {
		f.complete()
	}
}

func (f *httpFilterInstance) OnDestroy(reason api.DestroyReason) {
	f.srv = nil
	f.complete = nil
}

func (f *httpFilterInstance) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	decoder := f.decodeHeaders(header)
	results := f.srv.ServeDecodeFilter(decoder)
	return results.Status
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	decoder := f.decodeData(buffer, endStream)
	results := f.srv.ServeDecodeFilter(decoder)
	return results.Status
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	encoder := f.encodeHeaders(header)
	results := f.srv.ServeEncodeFilter(encoder)
	return results.Status
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	encoder := f.encodeData(buffer, endStream)
	results := f.srv.ServeEncodeFilter(encoder)
	return results.Status
}

func (f *httpFilterInstance) decodeHeaders(header api.RequestHeaderMap) HttpFilterDecoderFunc {
	return func(c Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		c.SetRequestHeader(header)

		if err := p.HandleOnRequestHeader(c); err != nil {
			return ActionContinue, err
		}

		if c.IsRequestBodyWriteable() {
			return ActionPause, nil
		}

		return ActionContinue, nil
	}
}

func (f *httpFilterInstance) decodeData(buffer api.BufferInstance, endStream bool) HttpFilterDecoderFunc {
	return func(c Context, p HttpFilterDecodeProcessor) (HttpFilterAction, error) {
		if !isRequestBodyAccessible(c) {
			return ActionSkip, nil
		}

		if buffer.Len() > 0 {
			c.SetRequestBody(buffer)
		}

		if endStream {
			return ActionContinue, p.HandleOnRequestBody(c)
		}

		return ActionPause, nil
	}
}

func (f *httpFilterInstance) encodeHeaders(header api.ResponseHeaderMap) HttpFilterEncoderFunc {
	return func(c Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		c.SetResponseHeader(header)

		if err := p.HandleOnResponseHeader(c); err != nil {
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

// Attention! Please be mindful of the Listener or Cluster per_connection_buffer_limit_bytes value
// when enabling the response body access on ConfigOptions (EnableResponseBodyRead or EnableResponseBodyWrite).
// The default value set by Envoy is 1MB. If the response body size exceeds this limit, the process will be halted.
// Although it's unclear whether this is considered a bug or a limitation at present,
// the Envoy Golang HTTP Filter library currently returns a 413 status code with a PayloadTooLarge message in such cases.
// Code references: https://github.com/envoyproxy/envoy/blob/v1.29.4/contrib/golang/filters/http/source/processor_state.cc#L362-L371.
func (f *httpFilterInstance) encodeData(buffer api.BufferInstance, endStream bool) HttpFilterEncoderFunc {
	return func(c Context, p HttpFilterEncodeProcessor) (HttpFilterAction, error) {
		if !isResponseBodyAccessible(c) {
			return ActionSkip, nil
		}

		if buffer.Len() > 0 {
			c.SetResponseBody(buffer)
		}

		if endStream {
			return ActionContinue, p.HandleOnResponseBody(c)
		}

		return ActionPause, nil
	}
}
