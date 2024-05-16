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
			opts := []ContextOption{
				WithContextConfig(config),
				WithContextLogger(log),
				WithHttpFilter(filter),
			}

			ctx, err := newContext(cb, opts...)
			if err != nil {
				log.Error(err, "initialization failed, filter ignored ...")
				return NoOpHttpFilter
			}

			return &httpFilterInstance{ctx: ctx}
		}
	}
}

var _ api.StreamFilter = &httpFilterInstance{}

type httpFilterInstance struct {
	api.PassThroughStreamFilter

	ctx Context
}

func (f *httpFilterInstance) OnLog() {
	runHttpFilterOnComplete(f.ctx)
}

func (f *httpFilterInstance) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	fn := newDecodeHeadersPhase(header)
	return f.ctx.ServeHTTPFilter(fn)
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	fn := newDecodeDataPhase(buffer, endStream)
	return f.ctx.ServeHTTPFilter(fn)
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	fn := newEncodeHeadersPhase(header)
	return f.ctx.ServeHTTPFilter(fn)
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	fn := newEncodeDataPhase(buffer, endStream)
	return f.ctx.ServeHTTPFilter(fn)
}

func (f *httpFilterInstance) OnDestroy(reason api.DestroyReason) {
	f.ctx = nil
}
