package main

import "github.com/ardikabs/gaetway"

func init() {
	gonvoy.RunHttpFilter(new(Filter), gonvoy.ConfigOptions{})
}

type Filter struct{}

func (Filter) Name() string {
	return "helloworld"
}

func (Filter) OnBegin(c gonvoy.RuntimeContext, ctrl gonvoy.HttpFilterController) error {
	c.Log().Info("Hello World from the helloworld HTTP filter")
	return nil
}

func (Filter) OnComplete(c gonvoy.Context) error {
	return nil
}

func main() {}
