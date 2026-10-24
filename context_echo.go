package gonvoy

import (
	"net/http"
	"net/url"
)

// EchoContext is the Echo-style handler surface over the bridge exchange.
// It wraps the live request, the recording writer, and the bridgeExchange
// hooks so filter code reads like an Echo handler with no Echo dependency.
//
// The name is EchoContext rather than Context because Context is already the
// bridge-called surface (context.go:55) used by bridgeFilter and
// DefaultErrorHandler. A follow-up can rename that surface and shorten this
// one once callers migrate.
//
// Like the bridge handlers it replaces, an EchoContext touches only Go
// memory on the handler goroutine: Next goes through the existing callNext
// path (bridge.go:473) and request mutations land through
// applyRequestMutations (bridgefilter.go:534). It adds no goroutine, no
// signal, and no Envoy C call.
type EchoContext interface {
	// Request returns the live request the handler sees. Convenience
	// accessors cover the common 90 percent; the request stays reachable
	// for escape hatches.
	Request() *http.Request

	// Next sends the request upstream through Terminal and parks until the
	// upstream response arrives, like next.ServeHTTP(w, r). It keeps
	// single-shot semantics: calling it twice is a no-op. Calling it after
	// writing a response is a bug and returns an ErrRuntime error instead
	// of picking a winner silently.
	Next() error

	// Header returns the named request header, like r.Header.Get.
	Header(key string) string

	// SetHeader sets a response header, like w.Header().Set.
	SetHeader(key, val string)

	// SetRequestHeader sets a request header, like r.Header.Set. The diff
	// lands on the live Envoy map through applyRequestMutations.
	SetRequestHeader(key, val string)

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
}

var _ EchoContext = (*echoContext)(nil)

// echoContext is the EchoContext implementation. w and r are the values the
// bridge passed in; ex is the live exchange resolved from r, or nil outside
// the bridge.
type echoContext struct {
	w  ResponseWriter
	r  *http.Request
	ex *bridgeExchange
}

// newEchoContext builds the Context view over (w, r). The exchange is
// resolved from r and may be nil outside the bridge; every method stays
// nil-safe in that case, mirroring the handler.go helpers.
func newEchoContext(w ResponseWriter, r *http.Request) *echoContext {
	var ex *bridgeExchange
	if e, ok := exchangeFrom(r); ok {
		ex = e
	}
	return &echoContext{w: w, r: r, ex: ex}
}

// Echo adapts an Echo-style function to a Handler so the bridge runtime,
// Use, and envoy.Handle stay unchanged. A nil return without Next is the
// local reply path (or upstream continue when nothing was written); a
// non-nil return without a recorded failure is surfaced through the
// exchange so the error handler maps it.
func Echo(fn func(EchoContext) error) Handler {
	if fn == nil {
		panic("gonvoy.Echo: fn should not be nil")
	}
	return HandlerFunc(func(w ResponseWriter, r *http.Request) {
		c := newEchoContext(w, r)
		if err := fn(c); err != nil {
			c.fail(err)
		}
	})
}

// Request returns the live request.
func (c *echoContext) Request() *http.Request { return c.r }

// Next sends the request upstream once through the same callNext path and
// reports the write-then-Next misuse as an error. Outside the bridge the
// terminal is a no-op and Next returns nil.
func (c *echoContext) Next() error {
	Terminal{}.ServeHTTP(c.w, c.r)
	if c.ex == nil {
		return nil
	}
	return c.ex.failErr
}

// Header returns the named request header.
func (c *echoContext) Header(key string) string { return c.r.Header.Get(key) }

// SetHeader sets a response header.
func (c *echoContext) SetHeader(key, val string) { c.w.Header().Set(key, val) }

// SetRequestHeader sets a request header.
func (c *echoContext) SetRequestHeader(key, val string) {
	if c.r.Header == nil {
		c.r.Header = make(http.Header)
	}
	c.r.Header.Set(key, val)
}

// Method returns the request method.
func (c *echoContext) Method() string { return c.r.Method }

// Path returns the request path.
func (c *echoContext) Path() string {
	if c.r.URL == nil {
		return ""
	}
	return c.r.URL.Path
}

// Host returns the request host.
func (c *echoContext) Host() string { return c.r.Host }

// SetHost sets the request host.
func (c *echoContext) SetHost(host string) { c.r.Host = host }

// SetPath sets the request path.
func (c *echoContext) SetPath(path string) {
	if c.r.URL == nil {
		c.r.URL = &url.URL{}
	}
	c.r.URL.Path = path
}

// SetMethod sets the request method.
func (c *echoContext) SetMethod(method string) { c.r.Method = method }

// Status returns the recorded response status.
func (c *echoContext) Status() int { return c.w.Status() }

// fail records a handler error so the bridge error handler maps it. It is a
// no-op outside the bridge and when a failure was already recorded, such as
// the write-then-Next path through callNext.
func (c *echoContext) fail(err error) {
	if c.ex == nil || err == nil || c.ex.failErr != nil {
		return
	}
	c.ex.fail(err)
}
