// This package holds only filter registration (Handle). Everything else
// a handler needs lives in the root gonvoy package as plain Go, so unit
// tests never import this package.
//
// DO NOT INCLUDE this package on to your package that requires unit-test
// The primarily reason as because this package required to call the Envoy (C) process.
// ref: https://github.com/envoyproxy/envoy/issues/36380
package envoy

import (
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

// HandleEcho onboards an Echo-style handler at runtime. It must be called
// inside func init. It is a one-line wrapper around Handle that adapts fn
// through gonvoy.Echo, so registration reads as Echo without importing Echo.
func HandleEcho(name string, fn func(gonvoy.EchoContext) error, opts ...gonvoy.Option) {
	Handle(name, gonvoy.Echo(fn), opts...)
}
