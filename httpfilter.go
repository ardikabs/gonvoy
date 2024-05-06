package gonvoy

import (
	"fmt"
	"net/http"

	gate "github.com/ardikabs/gonvoy/pkg/featuregate"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoyhttp "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

func RunHttpFilter(filter HttpFilter) {
	RunHttpFilterWithConfig(filter, nil)
}

func RunHttpFilterWithConfig(filter HttpFilter, filterConfig interface{}) {
	envoyhttp.RegisterHttpFilterConfigFactoryAndParser(filter.Name(), httpFilterFactory(filter), newConfigParser(filterConfig))
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
		config, ok := cfg.(Configuration)
		if !ok {
			panic(fmt.Sprintf("httpFilterFactory: unexpected config type '%T', expecting '%T'", cfg, config))
		}

		return func(callbacks api.FilterCallbackHandler) api.StreamFilter {
			ctx, err := newContext(callbacks, config)
			if err != nil {
				callbacks.Log(api.Error, fmt.Sprintf("httpFilter(%s): context initialization failed; %v; filter ignored...", httpFilter.Name(), err))
				return NoOpHttpFilter
			}

			newHttpFilterIface, err := util.NewFrom(httpFilter)
			if err != nil {
				callbacks.Log(api.Error, fmt.Sprintf("httpFilter(%s): instance initialization failed; %v; filter ignored...", httpFilter.Name(), err))
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
	if !gate.AllowRequestHeaderPhase() {
		return api.Continue
	}

	f.ctx.SetRequestHeader(header)
	status := f.ctx.ServeHttpFilter(OnRequestHeaderPhase)

	// when the request body can be modified, but the current phase is already committed,
	// then we should honor that the current phase expect to return immediately to the Envoy
	// therefore this must be skipped
	if !f.ctx.Committed() && f.ctx.CanModifyRequestBody() {
		return api.StopAndBuffer
	}

	return status
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	if !gate.AllowResponseHeaderPhase() {
		return api.Continue
	}

	f.ctx.SetResponseHeader(header)
	status := f.ctx.ServeHttpFilter(OnResponseHeaderPhase)

	// when the response body can be modified, but the current phase is already committed,
	// then we should honor that the current phase expect to return immediately to the Envoy
	// therefore this must be skipped
	if !f.ctx.Committed() && f.ctx.CanModifyResponseBody() {
		return api.StopAndBuffer
	}

	return status
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	if !gate.AllowRequestBodyPhase() || f.ctx.Committed() {
		return api.Continue
	}

	if buffer.Len() > 0 {
		f.ctx.SetRequestBody(buffer)
	}

	if endStream {
		return f.ctx.ServeHttpFilter(OnRequestBodyPhase)
	}

	return api.StopAndBuffer
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	if !gate.AllowResponseBodyPhase() || f.ctx.Committed() {
		return api.Continue
	}

	if buffer.Len() > 0 {
		f.ctx.SetResponseBody(buffer)
	}

	if endStream {
		return f.ctx.ServeHttpFilter(OnResponseBodyPhase)
	}

	return api.StopAndBuffer
}

func (f *httpFilterInstance) OnDestroy(reason api.DestroyReason) {
	f.ctx = nil
	f.httpFilter = nil
}

type PassthroughHttpFilterHandler struct{}

func (PassthroughHttpFilterHandler) Disable() bool                                        { return false }
func (PassthroughHttpFilterHandler) OnRequestHeader(c Context, header http.Header) error  { return nil }
func (PassthroughHttpFilterHandler) OnRequestBody(c Context, body []byte) error           { return nil }
func (PassthroughHttpFilterHandler) OnResponseHeader(c Context, header http.Header) error { return nil }
func (PassthroughHttpFilterHandler) OnResponseBody(c Context, body []byte) error          { return nil }
