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
	Name() string
	OnStart(c Context) error
	OnComplete(c Context) error
}

func httpFilterFactory(httpFilter HttpFilter) api.StreamFilterConfigFactory {
	if util.IsNil(httpFilter) {
		panic("httpFilterFactory: httpFilter shouldn't be a nil")
	}

	return func(cfg interface{}) api.StreamFilterFactory {
		config, ok := cfg.(*globalConfig)
		if !ok {
			panic(fmt.Sprintf("httpFilterFactory: unexpected config type '%T', expecting '%T'", cfg, config))
		}

		return func(callbacks api.FilterCallbackHandler) api.StreamFilter {
			ctx, err := newContext(callbacks, config)
			if err != nil {
				callbacks.Log(api.Error, fmt.Sprintf("httpFilter(%s): filter context initialization failed; %v; filter ignored...", httpFilter.Name(), err))
				return NoOpHttpFilter
			}

			newHttpFilterIface, err := util.NewFrom(httpFilter)
			if err != nil {
				callbacks.Log(api.Error, fmt.Sprintf("httpFilter(%s): filter instance initialization failed; %v; filter ignored...", httpFilter.Name(), err))
				return NoOpHttpFilter
			}

			newHttpFilter := newHttpFilterIface.(HttpFilter)
			if err := newHttpFilter.OnStart(ctx); err != nil {
				callbacks.Log(api.Error, fmt.Sprintf("httpFilter(%s): caught an error during OnStart; %v; filter ignored...", httpFilter.Name(), err))
				return NoOpHttpFilter
			}

			return &httpFilterInstance{
				ctx:        ctx,
				httpFilter: newHttpFilter,
			}
		}
	}
}

var _ api.StreamFilter = &httpFilterInstance{}

type httpFilterInstance struct {
	api.PassThroughStreamFilter

	ctx        Context
	httpFilter HttpFilter
}

func (f *httpFilterInstance) OnLog() {
	if err := f.httpFilter.OnComplete(f.ctx); err != nil {
		f.ctx.Log().Error(err, "httpFilter(%s): caught an error during OnComplete; %v", f.httpFilter.Name(), err)
	}
}

func (f *httpFilterInstance) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	ctrl := newRequestHeaderController(header)
	return f.ctx.ServeFilter(ctrl)
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	ctrl := newResponseHeaderController(header)
	return f.ctx.ServeFilter(ctrl)
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	ctrl := newRequestBodyController(buffer, endStream)
	return f.ctx.ServeFilter(ctrl)
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	ctrl := newResponseBodyController(buffer, endStream)
	return f.ctx.ServeFilter(ctrl)
}

func (f *httpFilterInstance) OnDestroy(reason api.DestroyReason) {
	f.ctx = nil
	f.httpFilter = nil
}
