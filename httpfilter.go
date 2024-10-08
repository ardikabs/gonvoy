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

		manager, err := buildHttpFilterManager(ctx, filterFactoryFunc)
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

func httpFilterOnComplete(ctx Context, filter HttpFilter) {
	if err := filter.OnComplete(ctx); err != nil {
		ctx.Log().Error(err, "failed to complete HTTP filter execution")
	}
}
