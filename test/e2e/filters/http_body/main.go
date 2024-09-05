package main

import (
	"github.com/ardikabs/gaetway"
)

func init() {
	gaetway.RunHttpFilter(new(BodyReadFilter), gaetway.ConfigOptions{
		FilterConfig: new(Config),

		DisableStrictBodyAccess: true,
		EnableRequestBodyRead:   true,
		EnableResponseBodyRead:  true,
	})

	gaetway.RunHttpFilter(new(BodyWriteFilter), gaetway.ConfigOptions{
		FilterConfig: new(Config),

		DisableStrictBodyAccess: true,
		EnableRequestBodyWrite:  true,
		EnableResponseBodyWrite: true,
	})

	gaetway.RunHttpFilter(new(Echoserver), gaetway.ConfigOptions{
		DisableStrictBodyAccess: true,
		EnableRequestBodyRead:   true,
	})
}

func main() {}
