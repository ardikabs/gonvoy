package gonvoy

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
)

// Handler processes an HTTP request and/or response, net/http style.
// Do work, write, or call the next handler; see ServeHTTP.
type Handler interface {
	// ServeHTTP handles the request, writing the response when it answers directly.
	// Calling the next handler in the chain sends the request upstream;
	// code after that call runs when the upstream response is available.
	// A handler that writes and also calls the next handler is a bug;
	// the bridge answers 500 and logs instead of picking a winner silently.
	ServeHTTP(w ResponseWriter, r *http.Request)
}

// HandlerFunc adapts an ordinary function to a Handler.
type HandlerFunc func(ResponseWriter, *http.Request)

// ServeHTTP calls f(w, r).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *http.Request) { f(w, r) }

// ResponseWriter receives the response a handler writes, net/http style.
// It is not http.ResponseWriter: the response may be Envoy's upstream
// response rather than a fresh server response.
type ResponseWriter interface {
	// Header returns the response headers to modify.
	Header() http.Header

	// Write writes the response body. A first Write implies status 200.
	Write([]byte) (int, error)

	// WriteHeader records the response status code. The first call wins.
	WriteHeader(statusCode int)

	// Status returns the recorded status code, or 0 if none was written.
	// Beyond net/http: the bridge and middleware use it to tell a
	// pass-through (no status yet) from an explicit answer.
	Status() int
}

// Config returns the typed plugin config for the request.
// Use the same type passed at registration. It errors when the request
// carries no config or a config of another type.
func Config[T any](r *http.Request) (T, error) {
	var zero T

	v := r.Context().Value(bridgeConfigKey{})
	if v == nil {
		return zero, fmt.Errorf("filter config is not available, %w", errInternalConfigNotFound)
	}

	typed, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("unexpected filter config type '%T', expecting '%T'", v, zero)
	}

	return typed, nil
}

// Attr fetches an Envoy attribute that is not an HTTP field,
// such as a cluster or route name. Values are snapshotted at each filter
// phase, so what you read is current as of the last phase boundary.
// It is not Header: use r.Host, r.Method, and r.URL for HTTP fields.
// Outside the bridge the default is returned.
func Attr(r *http.Request, name, defaultVal string) (string, error) {
	if h, ok := hooksOf(r); ok && h.attr != nil {
		if v, ok := h.attr(name); ok {
			return v, nil
		}
	}

	return defaultVal, nil
}

// Reroute makes Envoy pick the route again, honoring r.Host, r.Method,
// and r.URL changes the handler made. Outside the bridge it is a no-op.
// This is the per-request action; ReloadRoute is the registration option
// that turns on route reloading for the filter.
func Reroute(r *http.Request) {
	if h, ok := hooksOf(r); ok && h.reroute != nil {
		h.reroute()
	}
}

// Log returns the request logger. Outside the bridge it discards.
func Log(r *http.Request) logr.Logger {
	if h, ok := hooksOf(r); ok && h.log != nil {
		return h.log()
	}

	return logr.Discard()
}

// SharedCache returns the filter-wide cache. Use it when variable
// initialization is expensive or requires statefulness.
// Outside the bridge it returns nil.
func SharedCache(r *http.Request) Cache {
	if h, ok := hooksOf(r); ok && h.sharedCache != nil {
		return h.sharedCache()
	}

	return nil
}

// MetricKind identifies a queued metric operation.
type MetricKind int

const (
	// MetricCounterAdd adds Delta to a counter.
	MetricCounterAdd MetricKind = iota

	// MetricGaugeAdd adds Delta to a gauge.
	MetricGaugeAdd

	// MetricGaugeSet sets a gauge to Value.
	MetricGaugeSet
)

// MetricOp is one queued metric operation, flushed in order on callback threads.
type MetricOp struct {
	Kind   MetricKind
	Name   string
	Labels []string
	Delta  int64
	Value  uint64
}

// Counter records counter operations.
type Counter struct {
	emit   func(MetricOp)
	name   string
	labels []string
}

// Increment queues a counter addition. Outside the bridge it is dropped.
func (c Counter) Increment(delta int64) {
	if c.emit == nil {
		return
	}

	c.emit(MetricOp{Kind: MetricCounterAdd, Name: c.name, Labels: c.labels, Delta: delta})
}

// Gauge records gauge operations.
type Gauge struct {
	emit   func(MetricOp)
	name   string
	labels []string
}

// Increment queues a gauge addition. Outside the bridge it is dropped.
func (g Gauge) Increment(delta int64) {
	if g.emit == nil {
		return
	}

	g.emit(MetricOp{Kind: MetricGaugeAdd, Name: g.name, Labels: g.labels, Delta: delta})
}

// Set queues a gauge assignment. Outside the bridge it is dropped.
func (g Gauge) Set(value uint64) {
	if g.emit == nil {
		return
	}

	g.emit(MetricOp{Kind: MetricGaugeSet, Name: g.name, Labels: g.labels, Value: value})
}

// MetricsRecorder records metric intents for callback-side flush.
// Get it from Stats; do not build it directly.
type MetricsRecorder struct {
	emit func(MetricOp)
}

// Counter returns a counter recorder.
func (m MetricsRecorder) Counter(name string, labels ...string) Counter {
	return Counter{emit: m.emit, name: name, labels: labels}
}

// Gauge returns a gauge recorder.
func (m MetricsRecorder) Gauge(name string, labels ...string) Gauge {
	return Gauge{emit: m.emit, name: name, labels: labels}
}

// Stats returns the metric recorder for the request.
// Operations queue for callback-side flush; outside the bridge they drop.
func Stats(r *http.Request) MetricsRecorder {
	if h, ok := hooksOf(r); ok {
		return MetricsRecorder{emit: h.emitMetric}
	}

	return MetricsRecorder{}
}
