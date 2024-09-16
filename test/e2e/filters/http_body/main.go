package main

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(
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

	gonvoy.RunHttpFilter(
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

	gonvoy.RunHttpFilter(
		echoServerName,
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
