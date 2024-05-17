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
//		RunHttpFilter(new(UserhttpFilter), ConfigOptions{
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
	OnBegin(c RuntimeContext, i *Instance) error

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
			opts := []ContextOption{WithContextConfig(config), WithContextLogger(log)}
			ctx, err := newContext(cb, opts...)
			if err != nil {
				log.Error(err, "failed to initialize filter, ignoring filter ...")
				return NoOpHttpFilter
			}

			srv, err := createServer(ctx, filter)
			if err != nil {
				log.Error(err, "failed to start filter, ignoring filter ...")
				return NoOpHttpFilter
			}

			return &httpFilter{srv}
		}
	}
}

func newInstance() *Instance {
	return &Instance{
		errorHandler: DefaultErrorHandler,
	}
}

// Instance represents an instance of an HTTP Filter.
type Instance struct {
	errorHandler ErrorHandler
	first        HttpFilterProcessor
	last         HttpFilterProcessor
}

// SetErrorHandler sets a custom error handler for an HTTP Filter.
func (i *Instance) SetErrorHandler(handler ErrorHandler) {
	if util.IsNil(handler) {
		return
	}

	i.errorHandler = handler
}

// AddHandler adds an HTTP Filter Handler to the chain,
// which should be run during filter startup (HttpFilter.OnBegin).
// It's important to note the order when adding filter handlers.
// While HTTP requests follow FIFO sequences, HTTP responses follow LIFO sequences.
//
// Example usage:
//
//	func (f *UserFilter) OnBegin(c RuntimeContext, i *Instance) error {
//		...
//		i.AddHandler(handlerA)
//		i.AddHandler(handlerB)
//		i.AddHandler(handlerC)
//		i.AddHandler(handlerD)
//	}
//
// During HTTP requests, traffic flows from `handlerA -> handlerB -> handlerC -> handlerD`.
// During HTTP responses, traffic flows in reverse: `handlerD -> handlerC -> handlerB -> handlerA`.
func (h *Instance) AddHandler(handler HttpFilterHandler) {
	if util.IsNil(handler) || handler.Disable() {
		return
	}

	proc := newHttpFilterProcessor(handler)
	if h.first == nil {
		h.first = proc
		h.last = proc
		return
	}

	proc.SetPrevious(h.last)
	h.last.SetNext(proc)
	h.last = proc
}

type CompleteFunc func()

func createServer(c Context, filter HttpFilter) (HttpFilterServer, error) {
	ins := newInstance()

	iface, err := util.NewFrom(filter)
	if err != nil {
		return nil, fmt.Errorf("filter server creation failed, %w", err)
	}

	newFilter := iface.(HttpFilter)
	if err := newFilter.OnBegin(c, ins); err != nil {
		return nil, fmt.Errorf("filter server startup failed, %w", err)
	}

	completeFunc := func() { runOnComplete(newFilter, c) }

	return &httpFilterServer{
		ctx:          c,
		errorHandler: ins.errorHandler,
		decoder:      ins.first,
		encoder:      ins.last,
		completer:    completeFunc,
	}, nil
}

func runOnComplete(filter HttpFilter, ctx Context) {
	if err := filter.OnComplete(ctx); err != nil {
		ctx.Log().Error(err, "filter completion failed")
	}
}
