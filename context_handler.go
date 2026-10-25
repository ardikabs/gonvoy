package gonvoy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
)

var _ Context = (*handlerContext)(nil)

// handlerContext is the Context implementation. w and r are the values the
// bridge passed in; ex is the live exchange resolved from r, or nil outside
// the bridge.
type handlerContext struct {
	w  ResponseWriter
	r  *http.Request
	ex *bridgeExchange
}

// newHandlerContext builds the Context view over (w, r). The exchange is
// resolved from r and may be nil outside the bridge; every method stays
// nil-safe in that case, mirroring the handler.go helpers.
func newHandlerContext(w ResponseWriter, r *http.Request) *handlerContext {
	var ex *bridgeExchange
	if e, ok := exchangeFrom(r); ok {
		ex = e
	}
	if r != nil && r.Header == nil {
		r.Header = make(http.Header)
	}
	return &handlerContext{w: w, r: r, ex: ex}
}

// Request returns the live request.
func (c *handlerContext) Request() *http.Request { return c.r }

// Response returns a live view of the response. Header aliases the
// recording writer headers; StatusCode and Status report the recorded
// status. Only Header is live: the remaining fields are read-only
// snapshots, so answer through JSON, String, NoContent, and Write.
// The view is fresh per call because the upstream publication swaps
// the header map: a view cached before next detaches and writes to it
// are lost, so call Response again after next.
func (c *handlerContext) Response() *http.Response {
	if c.w == nil {
		return &http.Response{Header: make(http.Header)}
	}

	status := c.w.Status()
	resp := &http.Response{Header: c.w.Header(), StatusCode: status}
	if text := http.StatusText(status); text != "" {
		resp.Status = fmt.Sprintf("%03d %s", status, text)
	}

	switch {
	case c.ex == nil:
		// Outside the bridge Body stays nil; ResponseBody reports the
		// absence instead.
	case !c.ex.nextCalled.Load():
		resp.Body = &poisonRespReader{ex: c.ex, log: c.Log()}
	case c.ex.respReader != nil:
		resp.Body = c.ex.respReader
	}

	return resp
}

// forwardUpstream starts forwarding the filter processing to upstream once
// through the internal terminal and parks until the upstream response
// arrives. It keeps single-shot semantics: calling it twice is a no-op.
// Calling it after writing a response is a bug and returns an ErrRuntime
// error instead of proceeding silently. Outside the bridge the terminal is
// a no-op and forwardUpstream returns nil. It is unexported: user code
// proceeds by calling the next Handler in the Middleware chain, never
// directly.
func (c *handlerContext) forwardUpstream() error {
	terminal{}.ServeHTTP(c.w, c.r)
	if c.ex == nil {
		return nil
	}
	return c.ex.failErr
}

// Method returns the request method.
func (c *handlerContext) Method() string { return c.r.Method }

// Path returns the request path.
func (c *handlerContext) Path() string {
	if c.r.URL == nil {
		return ""
	}
	return c.r.URL.Path
}

// Host returns the request host.
func (c *handlerContext) Host() string { return c.r.Host }

// SetHost sets the request host.
func (c *handlerContext) SetHost(host string) { c.r.Host = host }

// SetPath sets the request path.
func (c *handlerContext) SetPath(path string) {
	if c.r.URL == nil {
		c.r.URL = &url.URL{}
	}
	c.r.URL.Path = path
}

// SetMethod sets the request method.
func (c *handlerContext) SetMethod(method string) { c.r.Method = method }

// Status returns the recorded response status, or 0 if none was
// written yet.
func (c *handlerContext) Status() int {
	if c.w == nil {
		return 0
	}

	return c.w.Status()
}

// JSON answers with a JSON body. A marshal failure is returned and writes
// nothing.
func (c *handlerContext) JSON(code int, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to encode JSON response, %w", err)
	}

	c.w.Header().Set(HeaderContentType, MIMEApplicationJSONCharsetUTF8)
	c.w.WriteHeader(code)
	_, err = c.w.Write(b)
	return err
}

// String answers with a plain-text body.
func (c *handlerContext) String(code int, s string) error {
	c.w.Header().Set(HeaderContentType, MIMETextPlainCharsetUTF8)
	c.w.WriteHeader(code)
	_, err := c.w.Write([]byte(s))
	return err
}

// NoContent answers with a status and no body.
func (c *handlerContext) NoContent(code int) error {
	c.w.WriteHeader(code)
	return nil
}

// Write appends p to the recorded response body without marking an
// explicit status.
func (c *handlerContext) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

// Bind decodes the request body into v. Only JSON content is bound; any
// other content type is a caller-mapped error, never a panic. A read
// failure already carries its cause, and a decode failure is returned with
// context.
func (c *handlerContext) Bind(v any) error {
	ct := ""
	if c.r != nil {
		ct = c.r.Header.Get(HeaderContentType)
	}
	if !strings.Contains(ct, MIMEApplicationJSON) {
		return fmt.Errorf("cannot bind content type %q, %w", ct, ErrOperationNotPermitted)
	}

	b, err := io.ReadAll(c.RequestBody())
	if err != nil {
		return fmt.Errorf("failed to read request body, %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("failed to decode JSON request body, %w", err)
	}

	return nil
}

// RequestBody returns the request body for reading. A nil request or body reads
// as empty; gating failures surface on Read.
func (c *handlerContext) RequestBody() io.Reader {
	if c.r == nil || c.r.Body == nil {
		return http.NoBody
	}

	return c.r.Body
}

// ResponseBody returns the upstream response body for reading. Before
// next it records a phase violation that answers 500 and halts
// forwarding; after next it returns the same reader as Response().Body,
// gated only by the body access option.
func (c *handlerContext) ResponseBody() (io.Reader, error) {
	if c.r == nil {
		return nil, fmt.Errorf("response body is not accessible in this phase, %w", ErrOperationNotPermitted)
	}

	if c.ex != nil && !c.ex.nextCalled.Load() {
		return nil, c.ex.failEarlyRespBody(c.Log())
	}

	return ResponseBody(c.r)
}

// Log returns the request logger.
func (c *handlerContext) Log() logr.Logger { return Log(c.r) }

// Stats returns the metric recorder for the request.
func (c *handlerContext) Stats() MetricsRecorder { return Stats(c.r) }

// Cache returns the filter-wide cache.
func (c *handlerContext) Cache() Cache { return SharedCache(c.r) }

// Attr fetches an Envoy attribute that is not an HTTP field.
func (c *handlerContext) Attr(name, def string) (string, bool) { return Attr(c.r, name, def) }

// Reroute makes Envoy pick the route again.
func (c *handlerContext) Reroute() { Reroute(c.r) }

// RefreshRoute re-fetches the route immediately for this request through
// the request context.
func (c *handlerContext) RefreshRoute() {
	if c.r == nil {
		return
	}

	RefreshRouteCache(c.r.Context())
}

// OverrideUpstream pins this request to one upstream host.
func (c *handlerContext) OverrideUpstream(host string, strict bool) {
	OverrideUpstreamHost(c.r, host, strict)
}

// Drain marks the downstream connection to be drained after the current
// request completes.
func (c *handlerContext) Drain() { DrainConnection(c.r) }

// Secret fetches a named secret from Envoy's secret manager.
func (c *handlerContext) Secret(name string) (string, bool) { return Secret(c.r, name) }

// TLS returns the downstream TLS connection snapshot for the request.
func (c *handlerContext) TLS() (TLSConnection, bool) { return TLS(c.r) }

// QueryParam returns the named query parameter. A nil request or URL
// reads as absent.
func (c *handlerContext) QueryParam(name string) string {
	if c.r == nil || c.r.URL == nil {
		return ""
	}

	return c.r.URL.Query().Get(name)
}

// ConfigOf returns the typed plugin config for the request seen by c.
// It is a generic function rather than a method because Go methods must
// have no type parameters; it delegates to Config with the wrapped
// request. Use the same type passed at registration.
func ConfigOf[T any](c Context) (T, error) {
	if c == nil {
		var zero T

		return zero, fmt.Errorf("filter config is not available, %w", errInternalConfigNotFound)
	}

	r := c.Request()
	if r == nil {
		var zero T

		return zero, fmt.Errorf("filter config is not available, %w", errInternalConfigNotFound)
	}

	return Config[T](r)
}

// fail records a handler error so the bridge error handler maps it. It is a
// no-op outside the bridge and when a failure was already recorded, such as
// the write-then-proceed path through callNext.
func (c *handlerContext) fail(err error) {
	if c.ex == nil || err == nil || c.ex.failErr != nil {
		return
	}
	c.ex.fail(err)
}
