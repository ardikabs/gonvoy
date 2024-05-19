package gonvoy

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// HttpFilterContext represents the context object for an HTTP filter.
// It provides methods to access and modify various aspects of the HTTP request and response.
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

type ContextOption func(c *context) error

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
		c.cache = cfg.internalCache
		c.metrics = newMetrics(cfg.metricCounter, cfg.metricGauge, cfg.metricHistogram)

		c.strictBodyAccess = cfg.strictBodyAccess
		c.requestBodyAccessRead = cfg.allowRequestBodyRead
		c.requestBodyAccessWrite = cfg.allowRequestBodyWrite
		c.responseBodyAccessRead = cfg.allowResponseBodyRead
		c.responseBodyAccessWrite = cfg.allowResponseBodyWrite
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
