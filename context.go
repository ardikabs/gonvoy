package gonvoy

import (
	"errors"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// RuntimeContext represents the runtime context for the filter in Envoy.
// It provides various methods to interact with the Envoy proxy and retrieve information about the HTTP traffic.
type RuntimeContext interface {
	// Property fetches an Envoy attribute based on https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes.
	// Currently, it only supports value that has a string-like format, work in progress for List/Map format.
	//
	Property(name, defaultVal string) (string, error)

	// GetProperty is a helper function to fetch Envoy attributes based on https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/attributes.
	// Currently, it only supports value that has a string-like format, work in progress for List/Map format.
	//
	// Deprecated: use Property instead.
	GetProperty(name, defaultVal string) (string, error)

	// Log provides a logger from the plugin to the Envoy Log. It accessible under Envoy `http` and/or `golang` component.
	// Additionally, only debug, info, and error log levels are being taken into account.
	// e.g., Envoy flag `--component-log-level http:{debug,info,warn,error,critical},golang:{debug,info,warn,error,critical}`
	//
	Log() logr.Logger

	// Metrics provides an interface for user to create their custom metrics.
	//
	Metrics() Metrics
}

// hostContext carries runtime host hooks used by the bridge filter.
// It is not part of the documented user surface; the bridge reaches it
// through mustCastToContext.
type hostContext interface {
	Context

	// LoadRequestHeaders is a low-level API, it loads HTTP request headers from Envoy during DecodeHeaders phase
	//
	LoadRequestHeaders(api.RequestHeaderMap)

	// LoadResponseHeaders is a low-level API, it loads HTTP response headers from Envoy during EncodeHeaders phase
	//
	LoadResponseHeaders(api.ResponseHeaderMap)
}

// Context is the bridge-called surface: the only methods the bridge
// filter, the default error handler, and handlers reach through.
// It extends the RuntimeContext interface with request loading,
// local replies, route reloading, cache access, and body gating.
type Context interface {
	RuntimeContext

	// LoadRequestHeaders is a low-level API, it loads HTTP request headers from Envoy during DecodeHeaders phase
	//
	LoadRequestHeaders(api.RequestHeaderMap)

	// LoadResponseHeaders is a low-level API, it loads HTTP response headers from Envoy during EncodeHeaders phase
	//
	LoadResponseHeaders(api.ResponseHeaderMap)

	// SendResponse dispatches a response with a specified status code, body, and optional localreply options.
	// Use the JSON() method when you need to respond with a JSON content-type.
	// Use SendResponse only when you need to send a response with a custom content type.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	SendResponse(code int, bodyText string, opts ...LocalReplyOption) error

	// JSON dispatches a JSON response with a status code.
	// The default error handler answers through it.
	//
	// This action halts the handler chaining and immediately returns back to Envoy.
	JSON(code int, b []byte, opts ...LocalReplyOption) error

	// ReloadRoute reloads the route configuration, which basically re-evaluates the routing decisions.
	// You can enable this feature by setting the `RouteReloadable` field in the ConfigOptions.
	//
	ReloadRoute()

	// GetCache returns the filter-wide cache.
	GetCache() Cache

	// IsRequestBodyReadable reports whether the request body can be read.
	IsRequestBodyReadable() bool

	// IsRequestBodyWritable reports whether the request body can be modified.
	IsRequestBodyWritable() bool

	// IsResponseBodyReadable reports whether the response body can be read.
	IsResponseBodyReadable() bool

	// IsResponseBodyWritable reports whether the response body can be modified.
	IsResponseBodyWritable() bool
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
