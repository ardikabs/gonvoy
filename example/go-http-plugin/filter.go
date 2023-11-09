package main

import (
	"github.com/ardikabs/go-envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type filter struct {
	api.PassThroughStreamFilter

	callbacks api.FilterCallbackHandler
	config    *config

	ctx envoy.Context
}

func NewFilter(c *config, callbacks api.FilterCallbackHandler) *filter {
	ctx, err := envoy.NewContext(callbacks)
	if err != nil {
		panic(err)
	}

	f := &filter{
		ctx:       ctx,
		config:    c,
		callbacks: callbacks,
	}

	return f
}

func (f *filter) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	f.ctx.SetRequest(envoy.WithRequestHeaderMap(header))

	handlerOne := &HandlerOne{}
	handlerTwo := &HandlerTwo{}
	handlerThree := &HandlerThree{f.config.RequestHeaders}

	mgr := envoy.NewManager()
	mgr.Use(handlerOne.RequestHandler)
	mgr.Use(handlerTwo.RequestHandler)
	mgr.Use(handlerThree.RequestHandler)

	return mgr.Handle(f.ctx)
}

func (f *filter) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	f.ctx.SetResponse(envoy.WithResponseHeaderMap(header))

	handlerOne := &HandlerOne{}
	handlerTwo := &HandlerTwo{}
	handlerThree := &HandlerThree{f.config.RequestHeaders}

	mgr := envoy.NewManager()
	mgr.Use(handlerOne.ResponseHandler)
	mgr.Use(handlerTwo.ResponseHandler)
	mgr.Use(handlerThree.ResponseHandler)

	return mgr.Handle(f.ctx)

}

func (f *filter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	return api.Continue
}

func (f *filter) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	return api.Continue
}
