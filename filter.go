package envoy

import (
	"fmt"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoyhttp "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpFilter = &api.PassThroughStreamFilter{}

type HttpFilter interface {
	Name() string

	OnStart(c Context)
	Handlers(c Context) []HttpFilterHandler
	OnComplete(c Context)
}

type HttpFilterHandler interface {
	Disable() bool

	OnRequestHeader(c Context, header http.Header) error
	OnRequestBody(c Context, body []byte) error
	OnResponseHeader(c Context, header http.Header) error
	OnResponseBody(c Context, body []byte) error
}

func RunHttpFilter(filter HttpFilter, cfg interface{}) {
	envoyhttp.RegisterHttpFilterConfigFactoryAndParser(filter.Name(), filterInstanceFactory(filter), newConfigParser(cfg))
}

func filterInstanceFactory(filter HttpFilter) api.StreamFilterConfigFactory {
	return func(cfg interface{}) api.StreamFilterFactory {
		config, ok := cfg.(Configuration)
		if !ok {
			panic(fmt.Sprintf("filterfactory: unexpected config type, %T", cfg))
		}

		return func(callbacks api.FilterCallbackHandler) api.StreamFilter {
			ctx, err := NewContext(callbacks, WithFilterConfiguration(config))
			if err != nil {
				return NoOpFilter
			}

			return &filterInstance{
				ctx:        ctx,
				httpFilter: filter,
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
	f.httpFilter.OnStart(f.ctx)
}

func (f *filterInstance) OnLog() {
	f.httpFilter.OnComplete(f.ctx)
}

func (f *filterInstance) DecodeHeaders(header api.RequestHeaderMap, endStream bool) (status api.StatusType) {
	f.ctx.SetRequestHeader(header)

	mgr := NewManager()
	for _, handler := range f.httpFilter.Handlers(f.ctx) {
		handler := handler
		mgr.Use(func(next HandlerFunc) HandlerFunc {
			if handler.Disable() {
				return nil
			}

			return func(c Context) error {
				if err := handler.OnRequestHeader(c, c.Request().Header); err != nil {
					return err
				}
				return next(c)
			}
		})

	}

	return mgr.Handle(f.ctx)
}

func (f *filterInstance) DecodeData(api.BufferInstance, bool) api.StatusType {
	return api.Continue
}

func (f *filterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) (status api.StatusType) {
	f.ctx.SetResponseHeader(header)

	mgr := NewManager()
	for _, handler := range f.httpFilter.Handlers(f.ctx) {
		handler := handler
		mgr.Use(func(next HandlerFunc) HandlerFunc {
			if handler.Disable() {
				return nil
			}

			return func(c Context) error {
				if err := handler.OnResponseHeader(c, c.Response().Header); err != nil {
					return err
				}

				return next(c)
			}
		})
	}
	return mgr.Handle(f.ctx)
}

func (f *filterInstance) EncodeData(api.BufferInstance, bool) api.StatusType {
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
