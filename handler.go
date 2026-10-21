package gonvoy

import (
	stdcontext "context"
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
// The boolean reports whether the attribute exists; when it does not,
// the default is returned with false. Outside the bridge the default
// is returned, also with false.
func Attr(r *http.Request, name, defaultVal string) (string, bool) {
	if h, ok := hooksOf(r); ok && h.attr != nil {
		if v, ok := h.attr(name); ok {
			return v, true
		}
	}

	return defaultVal, false
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

// RefreshRouteCache re-fetches the route immediately for this request,
// through the standard library context rather than the request. It is the
// advanced, native-semantic counterpart to Reroute: use it only when no
// same-callback mutations are pending, as a synchronous re-fetch races
// Go-side header and path propagation and can resolve the stale route.
// If the handler mutated the request, prefer Reroute, whose lazy
// re-evaluation happens after the mutations land. Outside the bridge it
// is a no-op.
func RefreshRouteCache(ctx stdcontext.Context) {
	if ex, ok := ctx.Value(bridgeExchangeKey{}).(*bridgeExchange); ok && ex != nil {
		ex.refreshRoute = true
	}
}

// OverrideUpstreamHost pins this request to one upstream host, bypassing
// the load balancer. host must be an IP address with a port, such as
// "10.0.0.9:8080". When strict is true and that host is unavailable,
// Envoy answers 503; otherwise it falls back to load balancing. An invalid
// host answers 500. Outside the bridge it is a no-op.
func OverrideUpstreamHost(r *http.Request, host string, strict bool) {
	if h, ok := hooksOf(r); ok && h.overrideUpstream != nil {
		h.overrideUpstream(host, strict)
	}
}

// DrainConnection marks the downstream connection to be drained after the
// current request completes: "Connection: close" on HTTP/1.x, GOAWAY on
// HTTP/2 and HTTP/3. Outside the bridge it is a no-op.
func DrainConnection(r *http.Request) {
	if h, ok := hooksOf(r); ok && h.drain != nil {
		h.drain()
	}
}

// Secret fetches a named secret from Envoy's secret manager. Secrets are
// defined in the plugin configuration. The second return value is false
// when the secret is missing or the request runs outside the bridge.
func Secret(r *http.Request, name string) (string, bool) {
	if h, ok := hooksOf(r); ok && h.secret != nil {
		return h.secret(name)
	}

	return "", false
}

// TLSConnection is a snapshot of the downstream TLS connection, taken when
// the request headers arrive. String and slice fields are empty or nil when
// Envoy reports them unavailable; pointer fields are nil instead.
type TLSConnection struct {
	PeerCertificatePresented    bool
	PeerCertificateValidated    bool
	Sha256PeerCertificateDigest string
	SerialNumberPeerCertificate string
	SubjectPeerCertificate      string
	IssuerPeerCertificate       string
	SubjectLocalCertificate     string
	URISANsPeerCertificate      []string
	URISANsLocalCertificate     []string
	DNSSANsPeerCertificate      []string
	DNSSANsLocalCertificate     []string
	ValidFromPeerCertificate    *uint64
	ExpirationPeerCertificate   *uint64
	TLSVersion                  string
	Ciphersuite                 string
	CiphersuiteID               *uint16
	SessionID                   *string
	PEMPeerCertificate          string
	PEMPeerCertificateChain     string
}

// TLS returns the downstream TLS connection snapshot for the request.
// The second return value is false for plaintext connections and outside
// the bridge.
func TLS(r *http.Request) (TLSConnection, bool) {
	ex, ok := exchangeFrom(r)
	if !ok || !ex.tlsOK {
		return TLSConnection{}, false
	}

	return ex.tls, true
}
