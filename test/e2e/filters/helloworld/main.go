package main

import "github.com/ardikabs/gaetway"

func init() {
	gaetway.RunHttpFilter(new(Filter), gaetway.ConfigOptions{})
}

type Filter struct{}

func (Filter) Name() string {
	return "helloworld"
}

func (Filter) OnBegin(c gaetway.RuntimeContext, ctrl gaetway.HttpFilterController) error {
	c.Log().Info("Hello World from the helloworld HTTP filter")
	return nil
}

func (Filter) OnComplete(c gaetway.Context) error {
	return nil
}

func main() {}
