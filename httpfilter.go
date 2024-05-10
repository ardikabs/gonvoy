package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoyhttp "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

func RunHttpFilter(filter HttpFilter, options ConfigOptions) {
	envoyhttp.RegisterHttpFilterConfigFactoryAndParser(
		filter.Name(),
		httpFilterFactory(filter),
		newConfigParser(options),
	)
}

type HttpFilter interface {
	// Name used as the filter name on Envoy.
	//
	Name() string

	// OnBegin is executed during filter startup.
	// If an error is returned, the filter will be ignored.
	// This step could be used by the user to do filter preparation such as but not limited to:
	// retrieving filter configuration (if provided), register filter handlers, or capture user-generated metrics.
	//
	OnBegin(c RuntimeContext) error

	// OnComplete is executed filter completion.
	// If an error is returned, nothing happened.
	// This step could be used by the user to do filter completion such as but not limited to:
	// capture user-generated metrics, or resource cleanup.
	//
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
	strategy := newRequestHeaderStrategy(header)
	return f.ctx.ServeHTTPFilter(strategy)
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	strategy := newRequestBodyStrategy(buffer, endStream)
	return f.ctx.ServeHTTPFilter(strategy)
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	strategy := newResponseHeaderStrategy(header)
	return f.ctx.ServeHTTPFilter(strategy)
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	strategy := newResponseBodyStrategy(buffer, endStream)
	return f.ctx.ServeHTTPFilter(strategy)
}

func (f *httpFilterInstance) OnDestroy(reason api.DestroyReason) {
	f.ctx = nil
}
