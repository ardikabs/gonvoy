package gonvoy

import (
	"fmt"
	"net/http"

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
			metrics := newMetrics(config.metricCounter, config.metricGauge, config.metricHistogram)
			ctx, err := newContext(callbacks,
				WithConfiguration(config),
				WithMetricHandler(metrics),
				WithHttpFilterPhaseRules(config.enabledHttpFilterPhases, config.disabledHttpFilterPhases),
			)
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
	phase := OnRequestHeaderPhase
	isPhaseNotAllowed := !f.ctx.IsHttpFilterPhaseEnabled(phase) || f.ctx.IsHttpFilterPhaseDisabled(phase)
	if isPhaseNotAllowed {
		return api.Continue
	}

	f.ctx.SetRequestHeader(header)
	status := f.ctx.ServeHttpFilter(phase)

	nextPhase := OnRequestBodyPhase
	isNextPhaseAllowed := f.ctx.IsHttpFilterPhaseEnabled(nextPhase) || !f.ctx.IsHttpFilterPhaseDisabled(nextPhase)
	if isNextPhaseAllowed && f.ctx.IsRequestBodyWriteable() {
		f.ctx.RequestHeader().Del(HeaderXContentOperation)
		return api.StopAndBuffer
	}

	return status
}

func (f *httpFilterInstance) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	phase := OnRequestBodyPhase
	isPhaseAllowed := f.ctx.IsHttpFilterPhaseEnabled(phase) || !f.ctx.IsHttpFilterPhaseDisabled(phase)
	isBodyAccessible := f.ctx.IsRequestBodyReadable() || f.ctx.IsRequestBodyWriteable()
	if !isPhaseAllowed && !isBodyAccessible {
		return api.Continue
	}

	if buffer.Len() > 0 {
		f.ctx.SetRequestBody(buffer)
	}

	if endStream {
		return f.ctx.ServeHttpFilter(phase)
	}

	return api.StopAndBuffer
}

func (f *httpFilterInstance) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	phase := OnResponseHeaderPhase
	isPhaseAllowed := f.ctx.IsHttpFilterPhaseEnabled(phase) || !f.ctx.IsHttpFilterPhaseDisabled(phase)
	if !isPhaseAllowed {
		return api.Continue
	}

	f.ctx.SetResponseHeader(header)
	status := f.ctx.ServeHttpFilter(phase)

	nextPhase := OnResponseBodyPhase
	isNextPhaseAllowed := f.ctx.IsHttpFilterPhaseEnabled(nextPhase) || !f.ctx.IsHttpFilterPhaseDisabled(nextPhase)
	if isNextPhaseAllowed && f.ctx.IsResponseBodyWriteable() {
		f.ctx.ResponseHeader().Del(HeaderXContentOperation)
		return api.StopAndBuffer
	}

	return status
}

func (f *httpFilterInstance) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	phase := OnResponseBodyPhase
	isPhaseAllowed := f.ctx.IsHttpFilterPhaseEnabled(phase) || !f.ctx.IsHttpFilterPhaseDisabled(phase)
	isBodyAccessible := f.ctx.IsResponseBodyReadable() || f.ctx.IsResponseBodyWriteable()
	if !isPhaseAllowed && !isBodyAccessible {
		return api.Continue
	}

	isBodyNotAccessible := !(f.ctx.IsResponseBodyReadable() || f.ctx.IsResponseBodyWriteable())
	if isBodyNotAccessible {
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
