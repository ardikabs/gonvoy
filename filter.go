package envoy

import (
	"fmt"
	"net/http"

	"github.com/ardikabs/go-envoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoyhttp "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpFilter = &api.PassThroughStreamFilter{}

func RunHttpFilter(filter HttpFilter) {
	RunHttpFilterWithConfig(filter, nil)
}

func RunHttpFilterWithConfig(filter HttpFilter, filterConfig interface{}) {
	envoyhttp.RegisterHttpFilterConfigFactoryAndParser(filter.Name(), httpFilterFactory(filter), newConfigParser(filterConfig))
}

type HttpFilter interface {
	Name() string

	OnStart(c Context)
	RegisterHttpFilterHandler(c Context, mgr HandlerManager)
	OnComplete(c Context)
}

type HttpFilterHandler interface {
	Disable() bool

	OnRequestHeader(c Context, header http.Header) error
	OnRequestBody(c Context, body []byte) error
	OnResponseHeader(c Context, header http.Header) error
	OnResponseBody(c Context, body []byte) error
}

func httpFilterFactory(httpFilter HttpFilter) api.StreamFilterConfigFactory {
	return func(cfg interface{}) api.StreamFilterFactory {
		config, ok := cfg.(Configuration)
		if !ok {
			panic(fmt.Sprintf("filterfactory: unexpected config type, %T", cfg))
		}

		return func(callbacks api.FilterCallbackHandler) api.StreamFilter {
			ctx, err := NewContext(callbacks, WithFilterConfiguration(config))
			if err != nil {
				callbacks.Log(api.Critical, fmt.Sprintf("httpFilterFactory: failed during filter context initialization; %v", err))
				callbacks.Log(api.Info, fmt.Sprintf("httpFilterFactory: filter '%s' is ignored", httpFilter.Name()))
				return NoOpFilter
			}

			newHttpFilter, err := util.NewFrom(httpFilter)
			if err != nil {
				callbacks.Log(api.Critical, fmt.Sprintf("httpFilterFactory: failed during filter instance initialization; %v", err))
				callbacks.Log(api.Info, fmt.Sprintf("httpFilterFactory: filter '%s' is ignored", httpFilter.Name()))
				return NoOpFilter
			}

			return &filterInstance{
				ctx:        ctx,
				httpFilter: newHttpFilter.(HttpFilter),
			}
		}
	}
}

var _ api.StreamFilter = &filterInstance{}

type filterInstance struct {
	api.PassThroughStreamFilter

	ctx        Context
	httpFilter HttpFilter
}

// I'm still uncertain why this method is never called.
func (f *filterInstance) OnLogDownstreamStart() {
	fmt.Println("onlogdownstreamstart just now")

	f.httpFilter.OnStart(f.ctx)
}

func (f *filterInstance) OnLog() {
	f.httpFilter.OnComplete(f.ctx)
}

func (f *filterInstance) DecodeHeaders(header api.RequestHeaderMap, endStream bool) (status api.StatusType) {
	f.ctx.SetRequestHeader(header)

	manager := newHandlerManager()
	f.httpFilter.RegisterHttpFilterHandler(f.ctx, manager)
	return manager.handle(f.ctx, OnRequestHeaderPhase)
}

func (f *filterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) (status api.StatusType) {
	f.ctx.SetResponseHeader(header)

	manager := newHandlerManager()
	f.httpFilter.RegisterHttpFilterHandler(f.ctx, manager)
	return manager.handle(f.ctx, OnResponseHeaderPhase)
}

func (f *filterInstance) DecodeData(buffer api.BufferInstance, endStream bool) (status api.StatusType) {
	return api.Continue
}

func (f *filterInstance) EncodeData(buffer api.BufferInstance, endStream bool) (status api.StatusType) {
	return api.Continue
}

func (f *filterInstance) OnDestroy(reason api.DestroyReason) {
	f.ctx = nil
	f.httpFilter = nil
}

type PassthroughHttpFilterHandler struct{}

func (PassthroughHttpFilterHandler) Disable() bool                                        { return false }
func (PassthroughHttpFilterHandler) OnRequestHeader(c Context, header http.Header) error  { return nil }
func (PassthroughHttpFilterHandler) OnRequestBody(c Context, body []byte) error           { return nil }
func (PassthroughHttpFilterHandler) OnResponseHeader(c Context, header http.Header) error { return nil }
func (PassthroughHttpFilterHandler) OnResponseBody(c Context, body []byte) error          { return nil }
