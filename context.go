package envoy

import (
	"errors"
	"net/http"
	"runtime"
	"sync"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

type Context interface {
	// RequestHeader provides an interface to access and modify HTTP Request header, including
	// add, overwrite, or delete existing header.
	// RequestHeader will panic when it used without initialize the request header map first.
	//
	// See WithRequestHeaderMap.
	RequestHeader() Header

	// ResponseHeader provides an interface to access and modify HTTP Response header, including
	// add, overwrite, or delete existing header.
	// ResponseHeader will panic when it used without initialize the response header map first.
	//
	// See WithResponseHeaderMap.
	ResponseHeader() Header

	// BufferWriter provides an interface for interacting and modifying HTTP Request/Response body.
	// Additionally, the proper use of BufferWriter is contingent upon the Request/Response Header.
	// This means that it necessitates the Decode/Encode Headers phase before making use of BufferWriter.
	//
	// > Examples:
	// To initialize, you need to add during Decode/Encode Data phase.
	//
	// // During Decode Data
	// func (f *filter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	//    f.ctx.SetRequest(WithBufferInstance(buffer))
	// }
	//
	// // During Encode Data
	// func (f *filter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	//    f.ctx.SetResponse(WithBufferInstance(buffer))
	// }
	BufferWriter() BufferWriter

	// Request returns an http.Request struct, which is a read-only data.
	// Attempting to modify this value will have no effect.
	// To make modifications to the request, such as its headers, please use the RequestHeader() method instead.
	Request() *http.Request

	// SetRequest is a low-level API, it set response from RequestHeaderMap interface
	SetRequest(opts ...ContextOption)

	// Response returns an http.Response struct, which is a read-only data.
	// It means, update anything to this value will result nothing.
	// To make modifications to the response, such as its headers, please use the ResponseHeader() method instead.
	Response() *http.Response

	// SetResponse is a low-level API, it set response from ResponseHeaderMap interface
	SetResponse(opts ...ContextOption)

	// Store allows you to save a value of any type under a key of any type.
	// Please be cautious! The Store function overwrites any existing data.
	Store(key any, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	// It returns true if a compatible value is successfully loaded,
	// and false if no value is found or an error occurs during the process.
	Load(key any, receiver interface{}) (ok bool, err error)

	// Log provides a logger from the plugin to the Envoy Log. It accessible under Envoy `http` component.
	// e.g., Envoy flag `--component-log-level http:{debug,info,warn,error,critical}`
	Log() logr.Logger

	// JSON sends a JSON response with status code.
	JSON(code int, b []byte, headers map[string]string, opts ...ReplyOption) error

	// String sends a plain text response with status code.
	String(code int, s string, opts ...ReplyOption) error

	// StatusType is a low-level API used to specify the type of status to be communicated to Envoy.
	StatusType() api.StatusType

	// Committed indicates whether the current context has already completed its processing
	// within the plugin and forwarded the result to Envoy.
	Committed() bool

	// StreamInfo offers an interface for retrieving comprehensive details about the incoming HTTP traffic, including
	// information such as the route name, filter chain name, dynamic metadata, and more.
	// It provides direct access to low-level Envoy information, so it's important to use it with a clear understanding of your intent.
	StreamInfo() api.StreamInfo

	// GetProperty fetch Envoy attribute and return the value as a string.
	// The list of attributes can be found in https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes.
	// If the fetch succeeded, a string will be returned.
	GetProperty(key string) (string, error)
}

type context struct {
	reqHeaderMap   api.RequestHeaderMap
	respHeaderMap  api.ResponseHeaderMap
	bufferInstance api.BufferInstance

	callback   api.FilterCallbacks
	statusType api.StatusType

	httpReq  *http.Request
	httpResp *http.Response

	storage sync.Map

	logger logr.Logger

	committed bool
}

func NewContext(callback api.FilterCallbacks) (Context, error) {
	if callback == nil {
		return nil, errors.New("callback MUST not nil")
	}

	return &context{
		callback: callback,
		logger:   NewLogger(callback),
	}, nil
}

func (c *context) StreamInfo() api.StreamInfo {
	return c.callback.StreamInfo()
}

func (c *context) Log() logr.Logger {
	return c.logger
}

func (c *context) JSON(code int, body []byte, headers map[string]string, opts ...ReplyOption) error {
	options := GetDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	if body == nil {
		body = []byte("{}")
	}

	headers["content-type"] = "application/json"
	c.callback.SendLocalReply(code, string(body), headers, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	runtime.GC()
	return nil
}

func (c *context) String(code int, s string, opts ...ReplyOption) error {
	options := GetDefaultReplyOptions()
	for _, opt := range opts {
		opt(options)
	}

	c.callback.SendLocalReply(code, s, map[string]string{}, options.grpcStatusCode, options.responseCodeDetails)
	c.committed = true
	c.statusType = options.statusType

	return nil
}

func (c *context) RequestHeader() Header {
	if c.reqHeaderMap == nil {
		panic("Request Header is not being initialized yet")
	}

	return &header{c.reqHeaderMap}
}

func (c *context) ResponseHeader() Header {
	if c.respHeaderMap == nil {
		panic("Response Header is not being initialized yet")
	}

	return &header{c.respHeaderMap}
}

func (c *context) BufferWriter() BufferWriter {
	if c.bufferInstance == nil {
		panic("Buffer Writer is not being initialized yet")
	}

	return &bufferWriter{c.bufferInstance}
}

func (c *context) SetRequest(opts ...ContextOption) {
	c.reset()

	for _, o := range opts {
		if err := o(c); err != nil {
			c.Log().Error(err, "set Http Request")
		}
	}
}

func (c *context) Request() *http.Request {
	if c.httpReq == nil {
		panic("Http Request is not yet initialized, see SetRequest.")
	}

	return c.httpReq
}

func (c *context) SetResponse(opts ...ContextOption) {
	c.reset()

	for _, o := range opts {
		if err := o(c); err != nil {
			c.Log().Error(err, "set Http Response")
		}
	}
}

func (c *context) Response() *http.Response {
	if c.httpResp == nil {
		panic("Http Response is not yet initialized, see SetResponse.")
	}

	return c.httpResp
}

func (c *context) StatusType() api.StatusType {
	return c.statusType
}

func (c *context) Committed() bool {
	return c.committed
}

func (c *context) GetProperty(key string) (string, error) {
	return c.callback.GetProperty(key)
}

func (c *context) reset() {
	c.statusType = api.Continue
	c.committed = false
}

func (c *context) Store(key any, value any) {
	c.storage.Store(key, value)
}

func (c *context) Load(key any, receiver interface{}) (bool, error) {
	if receiver == nil {
		return false, errors.New("context: receiver should not be nil")
	}

	v, ok := c.storage.Load(key)
	if !ok {
		return false, nil
	}

	if !CastTo(receiver, v) {
		return false, errors.New("context: receiver and value has an incompatible type")
	}

	return true, nil
}

type ReplyOptions struct {
	statusType          api.StatusType
	responseCodeDetails string
	grpcStatusCode      int64
}

type ReplyOption func(o *ReplyOptions)

func GetDefaultReplyOptions() *ReplyOptions {
	return &ReplyOptions{
		statusType:          api.LocalReply,
		grpcStatusCode:      -1,
		responseCodeDetails: "terminated from plugin",
	}
}

func WithResponseCodeDetails(detail string) ReplyOption {
	return func(o *ReplyOptions) {
		o.responseCodeDetails = detail
	}
}

func WithGrpcStatus(status int64) ReplyOption {
	return func(o *ReplyOptions) {
		o.grpcStatusCode = status
	}
}
