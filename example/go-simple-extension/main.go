package main

import (
	"go-simple-extension/httpfilter"

	"github.com/ardikabs/go-envoy"
)

func init() {
	envoy.RunHttpFilter(&httpfilter.Filter{}, httpfilter.Config{})
}

func main() {}
