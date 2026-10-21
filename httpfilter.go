package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

// HttpFilterFactory defines a factory func for creating an HttpFilter.
//
// It is passed to RunHttpFilter & buildHttpFilterManager and called in
// buildHttpFilterManager to create a new HTTP filter on each request.
//
// It should be defined within the init() func of the HTTP filter and
// must return a struct that satisfies HttpFilter. It may also be used
// to inject dependencies into this struct, if required by the handler.
type HttpFilterFactoryFunc func() HttpFilter

// HttpFilter defines an interface for an HTTP filter used in Envoy.
// It provides methods for managing filter names, startup, and completion.
// This interface is specifically designed as a mechanism for onboarding the user HTTP filters to Envoy.
// HttpFilter is always renewed for every request.
type HttpFilter interface {
	// OnBegin is executed during filter startup.
	//
	// The OnBegin method is called when the filter is initialized.
	// It can be used by the user to perform filter preparation tasks, such as:
	// - Retrieving filter configuration (if provided)
	// - Registering filter handlers
	// - Capturing user-generated metrics
	//
	// If an error is returned, the filter will be ignored.
	OnBegin(c RuntimeContext, ctrl HttpFilterController) error

	// OnComplete is executed when the filter is completed.
	//
	// The OnComplete method is called when the filter is about to be destroyed.
	// It can be used by the user to perform filter completion tasks, such as:
	// - Capturing user-generated metrics
	// - Cleaning up resources
	//
	// If an error is returned, nothing happens.
	OnComplete(c Context) error
}

func NewHttpFilterFactory(filterFactoryFunc HttpFilterFactoryFunc) api.StreamFilterFactory {
	if util.IsNil(filterFactoryFunc()) {
		panic("httpFilterFactory: filterFactoryFunc shouldn't return nil")
	}

	return streamFilterFactory(func(ctx Context) (*httpFilterManager, error) {
		return buildHttpFilterManager(ctx, filterFactoryFunc)
	})
}

// NewFilterChainFactory builds a StreamFilterFactory from a user Filter.
// A Composer builds its per-request chain through Use; any other Filter
// runs as a single filter.
func NewFilterChainFactory(newFilter func() Filter) api.StreamFilterFactory {
	if newFilter == nil || util.IsNil(newFilter()) {
		panic("filterChainFactory: newFilter shouldn't return nil")
	}

	return streamFilterFactory(func(ctx Context) (*httpFilterManager, error) {
		return buildFilterChainManager(ctx, newFilter)
	})
}

func streamFilterFactory(build func(Context) (*httpFilterManager, error)) api.StreamFilterFactory {
	return func(cfg interface{}, cb api.FilterCallbackHandler) api.StreamFilter {
		config, ok := cfg.(*internalConfig)
		if !ok {
			panic(fmt.Sprintf("httpFilterFactory: unexpected config type '%T', expecting '%T'", cfg, config))
		}

		logger := newLogger(cb)
		ctx, err := NewContext(cb, contextOptions{
			config: config,
			logger: logger,
		})
		if err != nil {
			logger.Error(err, "failed to initialize context for filter, ignoring filter ...")
			return NoOpHttpFilter
		}

		manager, err := build(ctx)
		if err != nil {
			logger.Error(err, "failed to build HTTP filter manager, ignoring filter ...")
			return NoOpHttpFilter
		}

		return &httpFilterImpl{manager}
	}
}

func buildHttpFilterManager(c Context, filterFactoryFunc HttpFilterFactoryFunc) (*httpFilterManager, error) {
	manager := newHttpFilterManager(c)

	newFilter := filterFactoryFunc()

	if err := newFilter.OnBegin(c, manager); err != nil {
		return nil, fmt.Errorf("failed to start HTTP filter, %w", err)
	}

	manager.completer = func() { httpFilterOnComplete(c, newFilter) }
	return manager, nil
}

// buildFilterChainManager builds a filter manager from a user Filter.
// A Composer builds its per-request chain through Use; any other Filter
// runs as a single filter. A Use error ignores the filter and logs,
// matching OnBegin failure. The composer owns position zero of its own
// chain: its implemented phases and OnComplete always run first, then the
// Use filters. Do not list the composer in its own Use slice.
func buildFilterChainManager(c Context, newFilter func() Filter) (*httpFilterManager, error) {
	manager := newHttpFilterManager(c)

	f := newFilter()
	if util.IsNil(f) {
		panic("filterChainManager: newFilter shouldn't return nil")
	}

	if comp, ok := f.(Composer); ok {
		filters, err := comp.Use(c)
		if err != nil {
			c.Log().Error(err, "failed to build HTTP filter chain, ignoring filter ...")
			return manager, nil
		}

		// The composer owns position zero of its own chain: its implemented
		// phases and OnComplete always run first, then the Use filters.
		// There is no way to skip yourself; leave methods unimplemented instead.
		manager.AddFilter(f)
		for _, sf := range filters {
			manager.AddFilter(sf)
		}

		return manager, nil
	}

	manager.AddFilter(f)
	return manager, nil
}

func httpFilterChainOnComplete(ctx Context, filters []Filter) {
	for _, f := range filters {
		cf, ok := f.(CompleteFilter)
		if !ok {
			continue
		}

		if err := cf.OnComplete(ctx); err != nil {
			ctx.Log().Error(err, "failed to complete HTTP filter execution")
		}
	}
}

func httpFilterOnComplete(ctx Context, filter HttpFilter) {
	if err := filter.OnComplete(ctx); err != nil {
		ctx.Log().Error(err, "failed to complete HTTP filter execution")
	}
}
