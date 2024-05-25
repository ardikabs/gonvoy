package main

import (
	"github.com/ardikabs/gonvoy"
)

func init() {
	gonvoy.RunHttpFilter(new(BodyReadFilter), gonvoy.ConfigOptions{
		FilterConfig: new(Config),

		DisableStrictBodyAccess: true,
		EnableRequestBodyRead:   true,
		EnableResponseBodyRead:  true,
	})

	gonvoy.RunHttpFilter(new(BodyWriteFilter), gonvoy.ConfigOptions{
		FilterConfig: new(Config),

		DisableStrictBodyAccess: true,
		EnableRequestBodyWrite:  true,
		EnableResponseBodyWrite: true,
	})

	gonvoy.RunHttpFilter(new(Echoserver), gonvoy.ConfigOptions{
		DisableStrictBodyAccess: true,
		EnableRequestBodyRead:   true,
	})
}

func main() {}
