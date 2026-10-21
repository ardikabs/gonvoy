// This package holds only filter registration (Handle, Register,
// MustRegister). Everything else a handler needs lives in the root gonvoy
// package as plain Go, so unit tests never import this package.
//
// DO NOT INCLUDE this package on to your package that requires unit-test
// The primarily reason as because this package required to call the Envoy (C) process.
// ref: https://github.com/envoyproxy/envoy/issues/36380
package envoy

import (
	"errors"
	"fmt"

	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

// Handle onboards a net/http handler at runtime. It must be called inside
// func init. The handler runs once per request; calling the terminal
// (directly or through Use) sends the request upstream.
func Handle(name string, h gonvoy.Handler, opts ...gonvoy.Option) {
	if util.IsNil(h) {
		panic("envoy.Handle: handler shouldn't be nil")
	}

	o := gonvoy.Options{}
	for _, opt := range opts {
		opt(&o)
	}

	http.RegisterHttpFilterFactoryAndConfigParser(
		name,
		gonvoy.NewHandlerChainFactory(h),
		gonvoy.NewConfigParser(gonvoy.ConfigOptionsFromOptions(o)),
	)
}

// Options configures a user HTTP filter registration.
type Options struct {
	// New builds the user Filter for each request. Required.
	New func() gonvoy.Filter

	// Config is the user filter configuration struct.
	Config any

	// MetricsPrefix specifies the prefix used for metrics.
	MetricsPrefix string

	// Route describes routing behavior.
	Route gonvoy.RoutePolicy

	// Merge describes parent/child merge behavior.
	Merge gonvoy.MergePolicy

	// Body describes HTTP body access.
	Body gonvoy.BodyPolicy
}

// Register onboards a user HTTP filter at runtime.
// It must be called inside `func init()` blocks in main package.
// Example usage:
//
//	package main
//	func init() {
//		Register("myfilter", Options{
//			New: func() gonvoy.Filter { return new(MyFilter) },
//		})
//	}
func Register(filterName string, opts Options) error {
	if opts.New == nil {
		return errors.New("envoy: Options.New must not be nil")
	}

	http.RegisterHttpFilterFactoryAndConfigParser(
		filterName,
		gonvoy.NewFilterChainFactory(opts.New),
		gonvoy.NewConfigParser(gonvoy.ConfigOptionsFromOptions(gonvoy.Options{
			Config:        opts.Config,
			MetricsPrefix: opts.MetricsPrefix,

			Route: opts.Route,
			Merge: opts.Merge,
			Body:  opts.Body,
		})),
	)

	return nil
}

// MustRegister is Register that panics on invalid options.
// It must be declared inside `func init()` blocks in main package.
func MustRegister(filterName string, opts Options) {
	if err := Register(filterName, opts); err != nil {
		panic(fmt.Sprintf("envoy: failed to register HTTP filter '%s', %v", filterName, err))
	}
}
