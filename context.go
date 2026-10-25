package gonvoy

import (
	"errors"
	"io"
	"net/http"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// Context is the handler surface over the bridge exchange.
// It wraps the live request, the recording writer, and the bridgeExchange
// hooks so filter code keeps handler code handler-oriented with no extra dependency.
//
// The bridge-called surface is the unexported bridgeContext used by
// bridgeFilter and the default error handler; user code sees only Context.
//
// Like the bridge handlers it replaces, a Context touches only Go
// memory on the handler goroutine: request mutations land through
// applyRequestMutations (bridgefilter.go:534). It adds no goroutine, no
// signal, and no Envoy C call. Proceeding upstream happens only through
// the Middleware chain: calling the next Handler parks until the upstream
// response arrives via the internal callNext path (bridge.go:473).
type Context interface {
	// Request returns the live request the handler sees. Convenience
	// accessors cover the common 90 percent; the request stays reachable
	// for escape hatches.
	Request() *http.Request

	// Response returns a live view of the response the handler writes.
	// Header aliases the recording writer headers, so Set, Add, Del, and
	// Get manipulate what reaches the client through the usual
	// bridge paths: pre-next writes merge or stash, post-next writes
	// diff against the upstream snapshot. On key conflicts upstream
	// always wins over pre-next writes; only post-next writes override
	// upstream. StatusCode and Status report
	// the recorded status. Only Header is live; StatusCode, Status,
	// Body, and all other fields are read-only snapshots, so answer
	// through JSON, String, NoContent, and Write instead of assigning
	// them. Body mirrors ResponseBody: nil outside the bridge, a failing
	// reader before next that records a 500-tracked phase violation, and
	// the live upstream reader after next. The view is fresh per call: a view cached before next detaches
	// when the upstream response publishes and writes to it are lost,
	// so call Response again after next.
	Response() *http.Response

	// Method returns the request method, like r.Method.
	Method() string

	// Path returns the request path, like r.URL.Path.
	Path() string

	// Host returns the request host, like r.Host.
	Host() string

	// SetHost sets the request host, like assigning r.Host.
	SetHost(host string)

	// SetPath sets the request path, like assigning r.URL.Path.
	SetPath(path string)

	// SetMethod sets the request method, like assigning r.Method.
	SetMethod(method string)

	// Status returns the recorded response status, or 0 if none was
	// written yet.
	Status() int

	// JSON answers with a JSON body, like the JSON reply helper. It
	// marshals v, sets the JSON content type, and records the status
	// through the recording writer.
	JSON(code int, v any) error

	// String answers with a plain-text body, like the String reply
	// helper. It sets the plain-text content type and records the
	// status through the recording writer.
	String(code int, s string) error

	// NoContent answers with a status and no body, recording the status
	// through the recording writer.
	NoContent(code int) error

	// Write appends response body bytes without marking an explicit
	// status, like writing the recording writer directly. The bridge
	// applies the usual after-next rules: a full local reply when there
	// is nothing to replace into, in-place replacement when the response
	// body is writable, or a permission error when it is not.
	Write(p []byte) (int, error)

	// Bind decodes the request body into v. It reads Body fully and
	// JSON-unmarshals when the request content type contains
	// application/json; any other content type returns a
	// ErrOperationNotPermitted-wrapped error for the caller to map to a
	// status. It never panics.
	Bind(v any) error

	// RequestBody returns the request body for reading. Reads go through
	// the gated reader, so without body access they fail with
	// ErrOperationNotPermitted.
	RequestBody() io.Reader

	// ResponseBody returns the upstream response body for reading. It is
	// only readable after next returns inside Use: touching it before
	// next records a phase violation that answers 500 and halts
	// forwarding with ErrRuntime. After next the only gate is the body
	// access option, reported as an ErrOperationNotPermitted-wrapped
	// error. It returns the same reader as Response().Body.
	ResponseBody() (io.Reader, error)

	// Log returns the request logger, like the Log helper. Outside the
	// bridge it discards.
	Log() logr.Logger

	// Stats returns the metric recorder for the request, like the Stats
	// helper. Operations queue for callback-side flush; outside the
	// bridge they drop.
	Stats() MetricsRecorder

	// Cache returns the filter-wide cache, like the SharedCache helper.
	// Outside the bridge it returns nil.
	Cache() Cache

	// Attr fetches an Envoy attribute that is not an HTTP field, like
	// the Attr helper. The boolean reports whether the attribute
	// exists; when it does not, the default is returned with false.
	Attr(name, def string) (string, bool)

	// Reroute makes Envoy pick the route again, like the Reroute
	// helper. Outside the bridge it is a no-op.
	Reroute()

	// RefreshRoute re-fetches the route immediately for this request,
	// like RefreshRouteCache with the request context. Prefer Reroute
	// when the handler mutated the request. Outside the bridge it is a
	// no-op.
	RefreshRoute()

	// OverrideUpstream pins this request to one upstream host, like
	// the OverrideUpstreamHost helper. Outside the bridge it is a
	// no-op.
	OverrideUpstream(host string, strict bool)

	// Drain marks the downstream connection to be drained after the
	// current request completes, like the DrainConnection helper.
	// Outside the bridge it is a no-op.
	Drain()

	// Secret fetches a named secret from Envoy's secret manager, like
	// the Secret helper. The second return value is false when the
	// secret is missing or the request runs outside the bridge.
	Secret(name string) (string, bool)

	// TLS returns the downstream TLS connection snapshot, like the TLS
	// helper. The second return value is false for plaintext
	// connections and outside the bridge.
	TLS() (TLSConnection, bool)

	// QueryParam returns the named query parameter, like
	// r.URL.Query().Get. It returns "" when the request or its URL is
	// nil, or when the parameter is absent.
	QueryParam(name string) string
}

// runtimeContext represents the runtime context for the filter in Envoy.
// It provides various methods to interact with the Envoy proxy and retrieve information about the HTTP traffic.
type runtimeContext interface {
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

// bridgeContext is the internal bridge-called surface: the only methods the bridge
// filter and the default error handler reach through.
// It extends the runtimeContext interface with request loading,
// local replies, route reloading, cache access, and body gating.
type bridgeContext interface {
	runtimeContext

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

// newBridgeContext creates a new context object for the filter.
func newBridgeContext(cb api.FilterCallbackHandler, o contextOptions) (bridgeContext, error) {
	if cb == nil {
		return nil, errors.New("filter callback can not be nil")
	}

	c := &context{
		cb: cb,
	}

	if err := o.apply(c); err != nil {
		return c, err
	}

	return c, nil
}

type context struct {
	cb  api.FilterCallbackHandler
	pcb api.FilterProcessCallbacks

	autoReloadRoute bool

	strictBodyAccess                bool
	requestBodyAccessRead           bool
	requestBodyAccessWrite          bool
	responseBodyAccessRead          bool
	responseBodyAccessWrite         bool
	preserveContentLengthOnRequest  bool
	preserveContentLengthOnResponse bool

	httpReq *http.Request

	filterConfig interface{}
	cache        Cache
	metrics      Metrics
	logger       logr.Logger
	committed    bool
}

func (c *context) reset() {
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
