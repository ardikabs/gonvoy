package main

import (
	"github.com/ardikabs/gonvoy"
	"github.com/ardikabs/gonvoy/pkg/envoy"
)

func init() {
	envoy.RegisterHttpFilter(
		bodyReadFilterName,
		func() gonvoy.HttpFilter {
			return new(BodyReadFilter)
		},
		gonvoy.ConfigOptions{
			FilterConfig: new(Config),

			DisableStrictBodyAccess: true,
			EnableRequestBodyRead:   true,
			EnableResponseBodyRead:  true,
		},
	)

	envoy.RegisterHttpFilter(
		bodyWriteFilterName,
		func() gonvoy.HttpFilter {
			return new(BodyWriteFilter)
		},
		gonvoy.ConfigOptions{
			FilterConfig: new(Config),

			DisableStrictBodyAccess: true,
			EnableRequestBodyWrite:  true,
			EnableResponseBodyWrite: true,
		},
	)

	envoy.RegisterHttpFilter(
		echoServerFilterName,
		func() gonvoy.HttpFilter {
			return new(Echoserver)
		},
		gonvoy.ConfigOptions{
			DisableStrictBodyAccess: true,
			EnableRequestBodyRead:   true,
		},
	)
}

func main() {}
