package gonvoy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/ardikabs/gonvoy/pkg/types"
	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

type HttpFilterContext interface {
	// RequestHeader provides an interface to access and modify HTTP Request header, including
	// add, overwrite, or delete existing header.
	// It returns panic when the filter has not yet traversed the HTTP request or OnRequestHeader phase is disabled.
	// Please refer to the previous Envoy's HTTP filter behavior.
	//
	RequestHeader() Header

	// ResponseHeader provides an interface to access and modify HTTP Response header, including
	// add, overwrite, or delete existing header.
	// It returns panic when OnResponseHeader phase is disabled or ResponseHeader accessed outside from the following phases:
	// OnResponseHeader, OnResponseBody
	//
	ResponseHeader() Header

	// RequestBodyBuffer provides an interface to access and manipulate an HTTP Request body.
	// It returns panic when OnRequestBody phase is disabled or RequestBody accessed outside from the following phases:
	// OnRequestBody, OnResponseHeader, OnResponseBody
	//
	RequestBody() Body

	// ResponseBody provides an interface access and manipulate an HTTP Response body.
	// It returns panic when OnResponseBody phase is disabled or ResponseBody accessed outside from the following phases:
	// OnResponseBody
	//
	ResponseBody() Body

	// Request returns an http.Request struct, which is a read-only data.
	// Attempting to modify this value will have no effect.
	// Refers to RequestHeader and RequestBody for modification attempts.
	// It returns panic when the filter has not yet traversed the HTTP request or OnRequestHeader phase is disabled.
	// Please refer to the previous Envoy's HTTP filter behavior.
	//
	Request() *http.Request

	// Response returns an http.Response struct, which is a read-only data.
	// Attempting to modify this value will have no effect.
	// Refers to ResponseHeader and ResponseBody for modification attempts.
	// It returns panic, when OnResponseHeader phase is disabled or Response accessed outside from the following phases:
	// OnResponseHeader, OnResponseBody
	//
	Response() *http.Response

	// SetRequestHeader is a low-level API, it set request header from RequestHeaderMap interface during DecodeHeaders phase
	//
	SetRequestHeader(api.RequestHeaderMap)

	// SetResponseHeader is a low-level API, it set response header from ResponseHeaderMap interface during EncodeHeaders phase
	//
	SetResponseHeader(api.ResponseHeaderMap)

	// SetRequestBody is a low-level API, it set request body from BufferInstance interface during DecodeData phase
	//
	SetRequestBody(api.BufferInstance)

	// SetResponseBody is a low-level API, it set response body from BufferInstance interface during EncodeData phase
	//
	SetResponseBody(api.BufferInstance)

	// IsRequestBodyReadable specifies whether an HTTP Request body is readable or not.
	//
	IsRequestBodyReadable() bool

	// IsRequestBodyWriteable specifies whether an HTTP Request body is writeable or not.
	//
	IsRequestBodyWriteable() bool

	// IsResponseBodyReadable specifies whether an HTTP Response body is readable or not.
	//
	IsResponseBodyReadable() bool

	// IsResponseBodyWriteable specifies whether an HTTP Response body is writeable or not.
	//
	IsResponseBodyWriteable() bool

	// JSON sends a JSON response with a status code.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	JSON(code int, b []byte, header http.Header, opts ...ReplyOption) error

	// String sends a plain text response with a status code.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	String(code int, s string, header http.Header, opts ...ReplyOption) error

	// SkipNextPhase immediately returns to the Envoy without further progressing to the next handler.
	// This action also enables users to bypass the next phase.
	// In HTTP request flows, invoking it from OnRequestHeader skips OnRequestBody phase.
	// In HTTP response flows, invoking it from OnResponseHeader skips OnResponseBody phase.
	//
	SkipNextPhase() error
}

type RuntimeContext interface {
	HttpFilterManager

	// GetProperty is a helper function to fetch Envoy attributes based on https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes.
	// Currently, it only supports value that has a string-like format, work in progress for List/Map format.
	//
	GetProperty(name, defaultVal string) (string, error)

	// StreamInfo offers an interface for retrieving comprehensive details about the incoming HTTP traffic, including
	// information such as the route name, filter chain name, dynamic metadata, and more.
	// It provides direct access to low-level Envoy information, so it's important to use it with a clear understanding of your intent.
	//
	StreamInfo() api.StreamInfo

	// GetFilterConfig returns the filter configuration associated with the route.
	// It defaults to the parent filter configuration if no route filter configuration was found.
	// Otherwise, once typed_per_filter_config present in the route then it will return the child filter configuration.
	// Whether these filter configurations can be merged depends on the filter configuration struct tags.
	//
	GetFilterConfig() interface{}

	// GlobalCache provides a global cache, that persists throughout Envoy's lifespan.
	// Use this cache when variable initialization is expensive or requires a statefulness.
	//
	GlobalCache() Cache

	// LocalCache provides a cache associated to a HTTP Context.
	// It is designed for sharing or moving data within an HTTP Context.
	// If you wish to share data throughout Envoy's lifespan, use GlobalCache instead.
	//
	LocalCache() Cache

	// Log provides a logger from the plugin to the Envoy Log. It accessible under Envoy `http` and/or `golang` component.
	// Additionally, only debug, info, and error log levels are being taken into account.
	// e.g., Envoy flag `--component-log-level http:{debug,info,warn,error,critical},golang:{debug,info,warn,error,critical}`
	//
	Log() logr.Logger

	// Metrics provides an interface for user to create their custom metrics.
	//
	Metrics() Metrics
}

type Context interface {
	RuntimeContext

	HttpFilterContext

	// StatusType is a low-level API used to specify the type of status to be communicated to Envoy.
	//
	StatusType() api.StatusType

	// Committed indicates whether the current context has already completed its processing
	// within the filter and forwarded the result to Envoy.
	//
	Committed() bool
}

type ContextOption func(c *context) error

func runHttpFilterOnComplete(c Context) {
	ctx, ok := c.(*context)
	if !ok {
		return
	}

	if ctx.filter == nil {
		return
	}

	if err := ctx.filter.OnComplete(c); err != nil {
		c.Log().Error(err, "filter completion failed")
	}
}

func WithHttpFilter(filter HttpFilter) ContextOption {
	return func(c *context) error {
		iface, err := util.NewFrom(filter)
		if err != nil {
			return fmt.Errorf("filter creation failed, %w", err)
		}

		newFilter := iface.(HttpFilter)
		if err := newFilter.OnBegin(c); err != nil {
			return fmt.Errorf("filter startup failed, %w", err)
		}

		c.filter = newFilter
		return nil
	}
}

func WithContextConfig(cfg *globalConfig) ContextOption {
	return func(c *context) error {
		type validator interface {
			Validate() error
		}

		if validate, ok := cfg.filterConfig.(validator); ok {
			if err := validate.Validate(); err != nil {
				return fmt.Errorf("invalid filter config; %w", err)
			}
		}

		c.filterConfig = cfg.filterConfig
		c.globalCache = cfg.globalCache
		c.metrics = newMetrics(cfg.metricCounter, cfg.metricGauge, cfg.metricHistogram)

		c.strictBodyAccess = cfg.strictBodyAccess
		c.requestBodyAccessRead = cfg.allowRequestBodyRead
		c.requestBodyAccessWrite = cfg.allowRequestBodyWrite
		c.responseBodyAccessRead = cfg.allowResponseBodyRead
		c.responseBodyAccessWrite = cfg.allowResponseBodyWrite

		c.manager = newHttpFilterManager(c)
		return nil
	}
}

func WithContextLogger(logger logr.Logger) ContextOption {
	return func(c *context) error {
		c.logger = logger
		return nil
	}
}

func newContext(cb api.FilterCallbacks, opts ...ContextOption) (Context, error) {
	if cb == nil {
		return nil, errors.New("filter callback can not be nil")
	}

	c := &context{
		callback:   cb,
		statusType: api.Continue,
		localCache: newCache(),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

type context struct {
	callback api.FilterCallbacks

	reqHeaderMap       api.RequestHeaderMap
	respHeaderMap      api.ResponseHeaderMap
	reqBufferInstance  api.BufferInstance
	respBufferInstance api.BufferInstance

	strictBodyAccess        bool
	requestBodyAccessRead   bool
	requestBodyAccessWrite  bool
	responseBodyAccessRead  bool
	responseBodyAccessWrite bool

	httpReq  *http.Request
	httpResp *http.Response

	filterConfig interface{}
	globalCache  Cache
	localCache   Cache
	metrics      Metrics
	logger       logr.Logger
	statusType   api.StatusType
	committed    bool

	manager HttpFilterManager
	filter  HttpFilter
}

func (c *context) JSON(code int, body []byte, header http.Header, opts ...ReplyOption) error {
	options := NewDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	if header == nil {
		header = make(http.Header)
	}

	if body == nil {
		body = []byte("{}")
	}

	header.Set("content-type", "application/json")
	c.callback.SendLocalReply(code, string(body), header, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	runtime.GC()
	return nil
}

func (c *context) String(code int, s string, header http.Header, opts ...ReplyOption) error {
	options := NewDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	c.callback.SendLocalReply(code, s, header, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	return nil
}

func (c *context) SkipNextPhase() error {
	c.statusType = api.Continue
	c.committed = true
	return nil
}

func (c *context) RequestHeader() Header {
	if c.reqHeaderMap == nil {
		panic("The Request Header is not initialized yet, likely because the filter has not yet traversed the HTTP request or OnRequestHeader is disabled. Please refer to the previous HTTP filter behavior.")
	}

	return &header{c.reqHeaderMap}
}

func (c *context) ResponseHeader() Header {
	if c.respHeaderMap == nil {
		panic("The Response Header is not initialized yet and is only available during the OnRequestHeader, OnRequestBody, and OnResponseHeader phases")
	}

	return &header{c.respHeaderMap}
}

func (c *context) RequestBody() Body {
	if c.reqBufferInstance == nil {
		panic("The Request Body is not initialized yet and is only accessible during the OnRequestBody, OnResponseHeader, and OnResponseBody phases.")
	}

	return &bodyWriter{
		writeable: c.IsRequestBodyWriteable(),
		header:    c.reqHeaderMap,
		buffer:    c.reqBufferInstance,
	}
}

func (c *context) ResponseBody() Body {
	if c.respBufferInstance == nil {
		panic("The Response Body is not initialized yet and is only accessible during OnResponseBody phase.")
	}

	return &bodyWriter{
		writeable: c.IsResponseBodyWriteable(),
		header:    c.respHeaderMap,
		buffer:    c.respBufferInstance,
	}
}

func (c *context) SetRequestHeader(header api.RequestHeaderMap) {
	c.reset()

	req, err := types.NewRequest(
		header.Method(),
		header.Host(),
		types.WithRequestURI(header.Path()),
		types.WithRequestHeaderRangeSetter(header),
	)
	if err != nil {
		c.Log().Error(err, "while initialize Http Request")
		return
	}

	c.httpReq = req
	c.reqHeaderMap = header

	c.requestBodyAccessRead, c.requestBodyAccessWrite = checkBodyAccessibility(c.strictBodyAccess, c.requestBodyAccessRead, c.requestBodyAccessWrite, header)

	if off := req.Header.Get(HeaderXRequestBodyAccess) == ValueXRequestBodyAccessOff; off {
		c.requestBodyAccessRead = !off && c.requestBodyAccessRead
		c.requestBodyAccessWrite = !off && c.requestBodyAccessWrite
	}
}

func (c *context) SetResponseHeader(header api.ResponseHeaderMap) {
	c.reset()

	code, ok := header.Status()
	if !ok {
		return
	}

	resp, err := types.NewResponse(code, types.WithResponseHeaderRangeSetter(header))
	if err != nil {
		c.Log().Error(err, "while initialize Http Response")
		return
	}

	c.httpResp = resp
	c.respHeaderMap = header

	c.responseBodyAccessRead, c.responseBodyAccessWrite = checkBodyAccessibility(c.strictBodyAccess, c.responseBodyAccessRead, c.responseBodyAccessWrite, header)

	if off := resp.Header.Get(HeaderXResponseBodyAccess) == ValueXResponseBodyAccessOff; off {
		c.responseBodyAccessRead = !off && c.responseBodyAccessRead
		c.responseBodyAccessWrite = !off && c.responseBodyAccessWrite
	}
}

func (c *context) SetRequestBody(buffer api.BufferInstance) {
	if c.reqBufferInstance == nil {
		c.reqBufferInstance = buffer
	}

	if buffer.Len() == 0 {
		return
	}

	buf := bytes.NewBuffer(buffer.Bytes())
	c.httpReq.Body = io.NopCloser(buf)
}

func (c *context) SetResponseBody(buffer api.BufferInstance) {
	if c.respBufferInstance == nil {
		c.respBufferInstance = buffer
	}

	if buffer.Len() == 0 {
		return
	}

	buf := bytes.NewBuffer(buffer.Bytes())
	c.httpResp.Body = io.NopCloser(buf)
}

func (c *context) Request() *http.Request {
	if c.httpReq == nil {
		panic("an HTTP Request is not initialized yet, likely because the filter has not yet traversed the HTTP request or OnRequestHeader is disabled. Please refer to the previous HTTP filter behavior.")
	}

	return c.httpReq
}

func (c *context) Response() *http.Response {
	if c.httpResp == nil {
		panic("an HTTP Response is not initialized yet, and is only available during the OnResponseHeader, and OnResponseBody phases.")
	}

	return c.httpResp
}

func (c *context) StatusType() api.StatusType {
	return c.statusType
}

func (c *context) Committed() bool {
	return c.committed
}

func (c *context) reset() {
	c.statusType = api.Continue
	c.committed = false
}

func (c *context) IsRequestBodyReadable() bool {
	// If the Request Body can be accessed, but the current phase has already been committed,
	// then Request Body is no longer accessible
	if c.committed {
		return false
	}

	return c.requestBodyAccessRead
}

func (c *context) IsRequestBodyWriteable() bool {
	// If the Request Body can be modified, but the current phase has already been committed,
	// then Request Body is no longer modifiable
	if c.committed {
		return false
	}

	return c.requestBodyAccessWrite
}

func (c *context) IsResponseBodyReadable() bool {
	// If the Response Body can be accessed, but the current phase has already been committed,
	// then Response Body is no longer accessible
	if c.committed {
		return false
	}

	return c.responseBodyAccessRead
}

func (c *context) IsResponseBodyWriteable() bool {
	// If the response body can be modified, but the current phase has already been committed,
	// then Response Body is no longer modifiable
	if c.committed {
		return false
	}

	return c.responseBodyAccessWrite
}

func (c *context) GetProperty(name, defaultVal string) (string, error) {
	value, err := c.callback.GetProperty(name)
	if err != nil {
		if errors.Is(err, api.ErrValueNotFound) {
			return defaultVal, nil
		}

		return value, err
	}

	if value == "" {
		return defaultVal, nil
	}

	return value, nil
}

func (c *context) StreamInfo() api.StreamInfo {
	return c.callback.StreamInfo()
}

func (c *context) GetFilterConfig() interface{} {
	return c.filterConfig
}

func (c *context) GlobalCache() Cache {
	return c.globalCache
}

func (c *context) LocalCache() Cache {
	return c.localCache
}

func (c *context) Log() logr.Logger {
	return c.logger
}

func (c *context) Metrics() Metrics {
	return c.metrics
}

func (c *context) SetErrorHandler(e ErrorHandler) {
	c.manager.SetErrorHandler(e)
}

func (c *context) RegisterHTTPFilterHandler(handler HttpFilterHandler) {
	c.manager.RegisterHTTPFilterHandler(handler)
}

func (c *context) ServeHTTPFilter(phase HttpFilterPhaseFunc) api.StatusType {
	return c.manager.ServeHTTPFilter(phase)
}
