package gonvoy

import (
	"errors"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// HttpFilterContext represents the context object for an HTTP filter.
// It provides methods to access and modify various aspects of the HTTP request and response.
type HttpFilterContext interface {
	// RequestHeader provides an interface to access and modify HTTP Request header, including
	// add, overwrite, or delete existing header.
	//
	// A panic is returned when the filter has not yet traversed the HTTP request or OnRequestHeader phase is disabled.
	// Please refer to the previous Envoy's HTTP filter behavior.
	//
	RequestHeader() Header

	// ResponseHeader provides an interface to access and modify HTTP Response header, including
	// add, overwrite, or delete existing header.
	//
	// A panic is returned when OnResponseHeader phase is disabled or ResponseHeader accessed outside from the following phases:
	// OnResponseHeader, OnResponseBody
	//
	ResponseHeader() Header

	// RequestBodyBuffer provides an interface to access and manipulate an HTTP Request body.
	//
	// A panic is returned when OnRequestBody phase is disabled or RequestBody accessed outside from the following phases:
	// OnRequestBody, OnResponseHeader, OnResponseBody
	//
	RequestBody() Body

	// ResponseBody provides an interface access and manipulate an HTTP Response body.
	//
	// A panic is returned when OnResponseBody phase is disabled or ResponseBody accessed outside from the following phases:
	// OnResponseBody
	//
	ResponseBody() Body

	// Request returns an http.Request struct, which is a read-only data.
	// Any attempts to alter this value will not affect to the actual request.
	// For any modifications, please use RequestHeader or RequestBody.
	//
	// Note: The request body only available during the OnRequestBody phase.
	//
	// A panic is returned if the filter has not yet traversed the HTTP request or the Request body access setting is turned off.
	// Please see to the previous Envoy's HTTP filter behavior.
	//
	Request() *http.Request

	// Response returns an http.Response struct, which is a read-only data.
	// Any attempts to alter this value will not affect to the actual response.
	// For any modifications, please use ResponseHeader or ResponseBody.
	//
	// Note: The response body only available during the OnResponseBody phase.
	//
	// A panic is returned if the Response body access setting is turned off or if Response is accessed outside of the following phases:
	// OnResponseHeader, or OnResponseBody
	//
	Response() *http.Response

	// SetRequestHost modifies the host of the request.
	// This can be used to dynamically change the request host based on specific conditions for routing.
	// However, to re-evaluate routing decisions, the filter must explicitly trigger ReloadRoute,
	// or the AutoReloadRouteOnHeaderChange option must be enabled in the ConfigOptions.
	//
	SetRequestHost(host string)

	// SetRequestMethod modifies the method of the request (GET, POST, etc.).
	// This can be used to dynamically change the request host based on specific conditions for routing.
	// However, to re-evaluate routing decisions, the filter must explicitly trigger ReloadRoute,
	// or the AutoReloadRouteOnHeaderChange option must be enabled in the ConfigOptions.
	//
	SetRequestMethod(method string)

	// SetRequestPath modifies the path of the request.
	// This can be used to dynamically change the request host based on specific conditions for routing.
	// However, to re-evaluate routing decisions, the filter must explicitly trigger ReloadRoute,
	// or the AutoReloadRouteOnHeaderChange option must be enabled in the ConfigOptions.
	//
	SetRequestPath(path string)

	// LoadRequestHeaders is a low-level API, it loads HTTP request headers from Envoy during DecodeHeaders phase
	//
	LoadRequestHeaders(api.RequestHeaderMap)

	// LoadResponseHeaders is a low-level API, it loads HTTP response headers from Envoy during EncodeHeaders phase
	//
	LoadResponseHeaders(api.ResponseHeaderMap)

	// LoadRequestBody is a low-level API, it loads HTTP request body from Envoy during DecodeData phase
	//
	LoadRequestBody(buffer api.BufferInstance, endStream bool)

	// LoadResponseBody is a low-level API, it loads HTTP response body from Envoy during EncodeData phase
	//
	LoadResponseBody(buffer api.BufferInstance, endStream bool)

	// IsRequestBodyAccessible checks if the request body is accessible for reading or writing.
	//
	IsRequestBodyAccessible() bool

	// IsRequestBodyReadable specifies whether an HTTP Request body is readable or not.
	//
	IsRequestBodyReadable() bool

	// IsRequestBodyWritable specifies whether an HTTP Request body is writeable or not.
	//
	IsRequestBodyWritable() bool

	// IsResponseBodyAccessible checks if the response body is accessible for reading or writing.
	//
	IsResponseBodyAccessible() bool

	// IsResponseBodyReadable specifies whether an HTTP Response body is readable or not.
	//
	IsResponseBodyReadable() bool

	// IsResponseBodyWritable specifies whether an HTTP Response body is writeable or not.
	//
	IsResponseBodyWritable() bool

	// SendResponse dispatches a response with a specified status code, body, and optional localreply options.
	// Use the JSON() method when you need to respond with a JSON content-type.
	// For plain text responses, use the String() method.
	// Use SendResponse only when you need to send a response with a custom content type.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	SendResponse(code int, bodyText string, opts ...LocalReplyOption) error

	// JSON dispatches a JSON response with a status code.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	JSON(code int, b []byte, opts ...LocalReplyOption) error

	// String dispatches a plain text response with a status code.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	String(code int, s string, opts ...LocalReplyOption) error

	// SkipNextPhase immediately returns to the Envoy without further progressing to the next handler.
	// This action also enables users to bypass the next phase.
	// In HTTP request flows, invoking it from OnRequestHeader skips OnRequestBody phase.
	// In HTTP response flows, invoking it from OnResponseHeader skips OnResponseBody phase.
	//
	SkipNextPhase() error

	// ReloadRoute reloads the route configuration, which basically re-evaluates the routing decisions.
	// You can enable this feature by setting the `RouteReloadable` field in the ConfigOptions.
	// Example use cases:
	// - When user wants to modify headers based on certain conditions, then later decides whether the request should be routed to a different cluster or upstream.
	// - When user wants to modify the routing decision based on the request body.
	//
	ReloadRoute()
}

// RuntimeContext represents the runtime context for the filter in Envoy.
// It provides various methods to interact with the Envoy proxy and retrieve information about the HTTP traffic.
type RuntimeContext interface {
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

	// GetCache returns a Cache.
	// Use this Cache when variable initialization is expensive or requires a statefulness.
	//
	GetCache() Cache

	// Log provides a logger from the plugin to the Envoy Log. It accessible under Envoy `http` and/or `golang` component.
	// Additionally, only debug, info, and error log levels are being taken into account.
	// e.g., Envoy flag `--component-log-level http:{debug,info,warn,error,critical},golang:{debug,info,warn,error,critical}`
	//
	Log() logr.Logger

	// Metrics provides an interface for user to create their custom metrics.
	//
	Metrics() Metrics
}

// Context represents the interface for a context within the filter.
// It extends the RuntimeContext and HttpFilterContext interfaces.
type Context interface {
	RuntimeContext

	HttpFilterContext

	// StatusType is a low-level API used to specify the type of status to be communicated to Envoy.
	//
	// Returns the status type.
	StatusType() api.StatusType

	// Committed indicates whether the current context has already completed its processing
	// within the filter and forwarded the result to Envoy.
	//
	// Returns true if the context has already committed, false otherwise.
	Committed() bool
}

// NewContext creates a new context object for the filter.
func NewContext(cb api.FilterCallbackHandler, o contextOptions) (Context, error) {
	if cb == nil {
		return nil, errors.New("filter callback can not be nil")
	}

	c := &context{
		cb:         cb,
		statusType: api.Continue,
	}

	if err := o.apply(c); err != nil {
		return c, err
	}

	return c, nil
}

type context struct {
	cb  api.FilterCallbackHandler
	pcb api.FilterProcessCallbacks

	reqHeaderMap       api.RequestHeaderMap
	respHeaderMap      api.ResponseHeaderMap
	reqBufferInstance  api.BufferInstance
	respBufferInstance api.BufferInstance
	reqBufferBytes     []byte
	respBufferBytes    []byte

	autoReloadRoute bool

	strictBodyAccess                bool
	requestBodyAccessRead           bool
	requestBodyAccessWrite          bool
	responseBodyAccessRead          bool
	responseBodyAccessWrite         bool
	preserveContentLengthOnRequest  bool
	preserveContentLengthOnResponse bool

	httpReq  *http.Request
	httpResp *http.Response

	filterConfig interface{}
	cache        Cache
	metrics      Metrics
	logger       logr.Logger
	statusType   api.StatusType
	committed    bool
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

type contextOptions struct {
	config *internalConfig
	logger logr.Logger
}

func (o *contextOptions) apply(c *context) error {
	if !o.logger.IsZero() {
		c.logger = o.logger
	}

	return applyInternalConfig(c, o.config)
}
