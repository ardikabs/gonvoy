package gonvoy

import (
	"fmt"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	envoy "github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

// RunHttpFilter is an entrypoint for onboarding User's HTTP filters at runtime.
// It must be declared inside `func init()` blocks in main package.
// Example usage:
//
//	package main
//	func init() {
//		RunHttpFilter(new(UserhttpFilter), ConfigOptions{
//			BaseConfig: new(UserHttpFilterConfig),
//		})
//	}
func RunHttpFilter(filter HttpFilter, options ConfigOptions) {
	envoy.RegisterHttpFilterConfigFactoryAndParser(
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
			ctx, err := NewContext(cb, WithContextConfig(config), WithContextLogger(log))
			if err != nil {
				log.Error(err, "failed to initialize context for filter, ignoring filter ...")
				return NoOpHttpFilter
			}

			manager, err := buildHttpFilterManager(ctx, filter)
			if err != nil {
				log.Error(err, "failed to build HTTP filter manager, ignoring filter ...")
				return NoOpHttpFilter
			}

			return &httpFilterImpl{manager}
		}
	}
}

func buildHttpFilterManager(c Context, filter HttpFilter) (*httpFilterManager, error) {
	manager := newHttpFilterManager(c)

	newFilter, err := createHttpFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP filter, %w", err)
	}

	if err := newFilter.OnBegin(c, manager); err != nil {
		return nil, fmt.Errorf("failed to start HTTP filter, %w", err)
	}

	manager.completer = func() { httpFilterOnComplete(c, newFilter) }
	return manager, nil
}

func createHttpFilter(filter HttpFilter) (HttpFilter, error) {
	iface, err := util.NewFrom(filter)
	if err != nil {
		return nil, err
	}

	return iface.(HttpFilter), nil
}

func httpFilterOnComplete(ctx Context, filter HttpFilter) {
	if err := filter.OnComplete(ctx); err != nil {
		ctx.Log().Error(err, "failed to complete HTTP filter execution")
	}
}
