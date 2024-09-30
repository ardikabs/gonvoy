package envoy

import (
	"github.com/ardikabs/gonvoy"
	"github.com/envoyproxy/envoy/contrib/golang/filters/http/source/go/pkg/http"
)

// RegisterHttpFilter is an entrypoint for onboarding User's HTTP filters at runtime.
// It must be declared inside `func init()` blocks in main package.
// Example usage:
//
//	package main
//	func init() {
//		RegisterHttpFilter(filterName, HttpFilterFactoryFunc, ConfigOptions{
//			BaseConfig: new(UserHttpFilterConfig),
//		})
//	}
func RegisterHttpFilter(filterName string, fn gonvoy.HttpFilterFactoryFunc, options gonvoy.ConfigOptions) {
	http.RegisterHttpFilterFactoryAndConfigParser(
		filterName,
		gonvoy.NewHttpFilterFactory(fn),
		gonvoy.NewConfigParser(options),
	)
}
