package main

import (
	"github.com/ardikabs/go-envoy/v1"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type filter struct {
	callbacks api.FilterCallbackHandler
	config    *config

	reqCtx envoy.RequestContext
	resCtx envoy.ResponseContext
}

func (f *filter) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	var err error
	f.reqCtx, err = envoy.NewRequestContext(header, f.callbacks)
	if err != nil {
		return api.Continue
	}

	handlerOne := &HandlerOne{}
	handlerTwo := &HandlerTwo{}
	handlerThree := &HandlerThree{f.config.RequestHeaders}

	handler := envoy.NewRequestHandler(f.reqCtx)
	handler.Use(handlerOne.RequestHandler)
	handler.Use(handlerTwo.RequestHandler)
	handler.Use(handlerThree.RequestHandler)
	return handler.Handle()
}

func (f *filter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	return api.Continue
}

func (f *filter) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	return api.Continue
}

func (f *filter) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	var err error
	f.resCtx, err = envoy.NewResponseContext(header, f.callbacks)
	if err != nil {
		return api.Continue
	}

	handlerOne := &HandlerOne{}
	handlerTwo := &HandlerTwo{}
	handlerThree := &HandlerThree{f.config.RequestHeaders}

	handler := envoy.NewResponseHandler(f.resCtx)
	handler.Use(handlerOne.ResponseHandler)
	handler.Use(handlerTwo.ResponseHandler)
	handler.Use(handlerThree.ResponseHandler)
	return handler.Handle()
}

func (f *filter) DecodeTrailers(trailers api.RequestTrailerMap) api.StatusType {
	return api.Continue
}

func (f *filter) EncodeTrailers(trailers api.ResponseTrailerMap) api.StatusType {
	return api.Continue
}

func (f *filter) OnDestroy(reason api.DestroyReason) {
}
