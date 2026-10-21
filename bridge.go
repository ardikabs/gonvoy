package gonvoy

import (
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// brSignal is a handler-goroutine to bridge-callback signal.
type brSignal int

const (
	// brNeedBody means the handler blocks reading the request body.
	brNeedBody brSignal = iota + 1
	// brProceed means next was called: continue upstream.
	brProceed
	// brDone means the handler returned. Outcome is on the exchange.
	brDone
	// brFailed means the handler panicked. The error is on the exchange.
	brFailed
)

// bridgeExchange carries one request through the coroutine bridge.
// The handler goroutine (G) touches only Go memory: snapshots, channels,
// the recording writer. Envoy C calls happen only on callback threads (W).
type bridgeExchange struct {
	handler Handler

	// w records the response. Pre-next writes are a local reply;
	// the terminal streams the upstream response into the passed writer.
	w *recordingWriter

	// req is the live request the handler sees.
	req *http.Request

	// gatedBody is r.Body while the bridge owns request reads.
	gatedBody *gatedBodyReader
	// respReader serves upstream response bytes to after-next code.
	// It is only fed when response body access is enabled.
	respReader *gatedRespReader

	// reqSnapshot holds request headers at DecodeHeaders, for diffing.
	reqSnapshot http.Header

	// origHost/origMethod/origPath snapshot routing state for reload checks.
	origHost   string
	origMethod string
	origPath   string

	// reqHeaderMap is the live Envoy map. W-side only, never touched by G.
	reqHeaderMap api.HeaderMap

	// preHeaders stashes pre-next response header sets for re-application.
	preHeaders http.Header

	// respHeader snapshots upstream response headers for diffing.
	respHeader http.Header

	// Outcome, written by G before signaling, read by W. Atomics first.
	nextCalled      atomic.Bool
	proceedUpstream atomic.Bool
	gDone           atomic.Bool

	// nextWriter is the writer passed to next; after-next reads and writes
	// go through it. Set once by G before signaling proceed.
	nextWriter ResponseWriter

	failErr      error
	replacedBody []byte
	reroute      bool
	// liveRespHeader is the Envoy response map for in-place mutation.
	liveRespHeader api.HeaderMap

	// Request body feed. W appends; G drains.
	bodyMu     sync.Mutex
	bodyChunks [][]byte
	bodyEnded  bool
	bodyWake   chan struct{}

	// Response body feed, used only when response body access is enabled.
	respMu     sync.Mutex
	respChunks [][]byte
	respEnded  bool
	respWake   chan struct{}

	// Response rendezvous. W closes respReady once headers are fed.
	respReady     chan struct{}
	respReadyOnce sync.Once
	respCode      int

	// Handshake. G signals; the waiting W consumes.
	sigCh chan brSignal

	// done closes on destroy. G selects on it everywhere it blocks.
	done     chan struct{}
	doneOnce sync.Once

	// Metrics intents, flushed on callback threads.
	metricMu  sync.Mutex
	metricOps []MetricOp

	// Attr snapshot, refreshed by W at every callback.
	attrMu sync.Mutex
	attrs  map[string]string

	// Hooks. Attr reads snapshots; reroute and emitMetric touch pure Go
	// state only. W applies the rest on callback threads.
	hooks bridgeHooks
}

// newBridgeExchange builds the exchange. Hooks may carry nil funcs when
// running outside the bridge (direct handler unit tests).
func newBridgeExchange(h Handler, req *http.Request, w *recordingWriter, hooks bridgeHooks) *bridgeExchange {
	return &bridgeExchange{
		handler:   h,
		w:         w,
		req:       req,
		bodyWake:  make(chan struct{}, 1),
		respWake:  make(chan struct{}, 1),
		respReady: make(chan struct{}),
		sigCh:     make(chan brSignal, 8),
		done:      make(chan struct{}),
		attrs:     make(map[string]string),
		hooks:     hooks,
	}
}

// signal sends to W without blocking. Signals back up while W waits;
// a full channel means W already has what it needs.
// signal sends a handler-goroutine signal to the waiting W thread.
// Body requests are lossy hints: W already holds the stream and a pending
// wake is enough. Terminal signals (proceed/done/failed) block until W
// receives or the stream dies, so a decision is never silently dropped.
func (ex *bridgeExchange) signal(s brSignal) {
	if s == brNeedBody {
		select {
		case ex.sigCh <- s:
		default:
		}

		return
	}

	select {
	case ex.sigCh <- s:
	case <-ex.done:
	}
}

// awaitSignal blocks W until G signals, the stream dies, or a safety
// timeout fires. G always has data when W waits, so a timeout means the
// handler is stuck: W returns paused and picks up G's decision, if any,
// at the next callback via drainDecision. This bounds the client-abort window.
//
// Timeout callers return StopNoBuffer on data paths (nothing new to buffer;
// the bytes that triggered the wait are already held) and StopAndBuffer on
// header paths (headers must stay buffered until the decision lands).
// Both converge at the next callback through drainDecision.
func (ex *bridgeExchange) awaitSignal() (brSignal, bool) {
	timeout := time.After(time.Second)

	select {
	case s := <-ex.sigCh:
		return s, true
	case <-ex.done:
		return 0, false
	case <-timeout:
		return 0, false
	}
}

// destroy unblocks the handler goroutine exactly once.
func (ex *bridgeExchange) destroy() {
	ex.doneOnce.Do(func() {
		close(ex.done)
	})
}

// feedBody appends a request body chunk. Safe for W to call anytime.
// It returns true when something actually changed, so callers only wake
// a blocked reader on progress: a wakeup always means progress and
// needBody is never signaled twice for the same wait.
func (ex *bridgeExchange) feedBody(p []byte, endStream bool) bool {
	changed := false

	ex.bodyMu.Lock()
	if len(p) > 0 {
		ex.bodyChunks = append(ex.bodyChunks, p)
		changed = true
	}
	if endStream && !ex.bodyEnded {
		ex.bodyEnded = true
		changed = true
	}
	ex.bodyMu.Unlock()

	if changed {
		ex.pingBody()
	}

	return changed
}

// feedRespBody appends a response body chunk. Safe for W to call anytime.
// Like feedBody, it reports whether anything changed.
func (ex *bridgeExchange) feedRespBody(p []byte, endStream bool) bool {
	changed := false

	ex.respMu.Lock()
	if len(p) > 0 {
		ex.respChunks = append(ex.respChunks, p)
		changed = true
	}
	if endStream && !ex.respEnded {
		ex.respEnded = true
		changed = true
	}
	ex.respMu.Unlock()

	if changed {
		select {
		case ex.respWake <- struct{}{}:
		default:
		}
	}

	return changed
}

// pingBody wakes a blocked reader. Non-blocking: a pending wake is enough.
func (ex *bridgeExchange) pingBody() {
	select {
	case ex.bodyWake <- struct{}{}:
	default:
	}
}

// Close is a no-op; the bridge owns the body lifecycle.
// gatedBodyReader is the request body the handler reads.
// Reads block G until bytes arrive, the stream ends, or it dies.
// It never touches Envoy; W feeds it. Every block signals W first,
// so a waiting W never misses a wakeup.
func (r *gatedBodyReader) Close() error { return nil }

type gatedBodyReader struct {
	ex      *bridgeExchange
	enabled bool
}

// gatedRespReader is the upstream response body the handler reads.
// It mirrors gatedBodyReader on the response side.
type gatedRespReader struct {
	ex      *bridgeExchange
	enabled bool
}

// Close is a no-op; the bridge owns the body lifecycle.
func (r *gatedRespReader) Close() error { return nil }

func (r *gatedRespReader) Read(p []byte) (int, error) {
	for {
		r.ex.respMu.Lock()
		if len(r.ex.respChunks) > 0 && r.enabled {
			chunk := r.ex.respChunks[0]
			r.ex.respChunks[0] = nil
			r.ex.respChunks = r.ex.respChunks[1:]
			r.ex.respMu.Unlock()

			n := copy(p, chunk)
			if n < len(chunk) {
				rest := append([]byte(nil), chunk[n:]...)
				r.ex.respMu.Lock()
				r.ex.respChunks = append([][]byte{rest}, r.ex.respChunks...)
				r.ex.respMu.Unlock()
			}
			return n, nil
		}

		ended := r.ex.respEnded
		r.ex.respMu.Unlock()

		if ended {
			return 0, io.EOF
		}

		if !r.enabled {
			return 0, fmt.Errorf("response body is not accessible in this phase, %w", ErrOperationNotPermitted)
		}

		r.ex.signal(brNeedBody)
		select {
		case <-r.ex.respWake:
		case <-r.ex.done:
			return 0, fmt.Errorf("request interrupted, %w", ErrClientClosedRequest)
		}
	}
}

// ResponseBody returns the upstream response body for reading.
// Bytes are available once the handler runs after next was called and
// response body access is enabled; otherwise reads fail immediately.
func ResponseBody(r *http.Request) (io.Reader, error) {
	ex, ok := exchangeFrom(r)
	if !ok || ex.respReader == nil {
		return nil, fmt.Errorf("response body is not accessible in this phase, %w", ErrOperationNotPermitted)
	}

	return ex.respReader, nil
}

func (r *gatedBodyReader) Read(p []byte) (int, error) {
	for {
		r.ex.bodyMu.Lock()
		if len(r.ex.bodyChunks) > 0 && r.enabled {
			chunk := r.ex.bodyChunks[0]
			r.ex.bodyChunks[0] = nil
			r.ex.bodyChunks = r.ex.bodyChunks[1:]
			r.ex.bodyMu.Unlock()

			n := copy(p, chunk)
			if n < len(chunk) {
				rest := append([]byte(nil), chunk[n:]...)
				r.ex.bodyMu.Lock()
				r.ex.bodyChunks = append([][]byte{rest}, r.ex.bodyChunks...)
				r.ex.bodyMu.Unlock()
			}
			return n, nil
		}

		ended := r.ex.bodyEnded
		r.ex.bodyMu.Unlock()

		if ended {
			return 0, io.EOF
		}

		if !r.enabled {
			return 0, fmt.Errorf("request body is not accessible in this phase, %w", ErrOperationNotPermitted)
		}

		r.ex.signal(brNeedBody)

		select {
		case <-r.ex.bodyWake:
		case <-r.ex.done:
			return 0, fmt.Errorf("request interrupted, %w", ErrClientClosedRequest)
		}
	}
}

// runHandler executes the middleware chain on G, converting the outcome
// to signals. Panics become brFailed; ErrOperationNotPermitted maps to
// 502 downstream through the default error handler.
func (ex *bridgeExchange) runHandler() {
	defer func() {
		ex.gDone.Store(true)
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				ex.failErr = err
			} else {
				ex.failErr = fmt.Errorf("handler panic: %v", r)
			}
			ex.signal(brFailed)
			return
		}
		ex.signal(brDone)
	}()

	ex.handler.ServeHTTP(ex.w, ex.req)
}

// callNext implements next.ServeHTTP for the terminal hop: it records the
// intent to proceed, reads a replaced request body if the handler assigned
// one, and parks G until W feeds the response. Calling next after writing
// a response body or status is a bug: the bridge answers 500 instead of
// picking a winner silently. Calling next twice is a no-op.
//
// A replacement body is read fully into memory; replacements are expected
// to be small buffered payloads, not streams.
func (ex *bridgeExchange) callNext(w ResponseWriter, r *http.Request) {
	if ex.nextCalled.Swap(true) {
		return
	}
	ex.proceedUpstream.Store(true)

	// A response written before next is a competing answer. This checks
	// the handler's own writer; wrappers own their bytes.
	if ex.w.explicit || len(ex.w.body) > 0 {
		ex.fail(fmt.Errorf("handler wrote a response and called next, %w", ErrRuntime))
		return
	}

	if r.Body != nil {
		if gb, ok := r.Body.(*gatedBodyReader); !ok || gb != ex.gatedBody {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				ex.fail(fmt.Errorf("failed to read replacement request body, %w", err))
				return
			}

			ex.replacedBody = body
		}
	}

	ex.nextWriter = w

	// Stash pre-next response header sets from the passed writer; they are
	// re-applied on top of the upstream response. Response bytes written
	// before next were already rejected above.
	ex.preHeaders = cloneHeader(w.Header())
	for k := range w.Header() {
		delete(w.Header(), k)
	}

	ex.signal(brProceed)

	select {
	case <-ex.respReady:
	case <-ex.done:
		return
	}

	// Write the upstream snapshot into the passed writer so after-next code
	// reads status and headers through the same value it was given.
	ex.writeUpstream(w)
}

// writeUpstream publishes the upstream response snapshot into w.
func (ex *bridgeExchange) writeUpstream(w ResponseWriter) {
	if tw, ok := w.(terminalWriter); ok {
		tw.setTerminal(ex.respCode, ex.respHeader, ex.preHeaders)
		return
	}

	w.WriteHeader(ex.respCode)
	for k, v := range ex.respHeader {
		w.Header()[k] = v
	}
	for k, v := range ex.preHeaders {
		w.Header()[k] = v
	}
}

// terminalWriter is implemented by recordingWriter to receive the upstream
// response without tripping the wrote flags user writes set.
type terminalWriter interface {
	ResponseWriter
	setTerminal(status int, upstream, stashed http.Header)
}

// fail records a terminal error and unblocks W to apply it.
func (ex *bridgeExchange) fail(err error) {
	ex.failErr = err
	ex.signal(brFailed)
}

// cloneHeader deep-copies a header map.
func cloneHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, v := range h {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// Handler runtime support. Everything below is unexported bridge machinery:
// the recording writer handlers write into, and the request-context hooks
// helpers resolve through. User code never names these.

// recordingWriter records what a handler writes for the bridge to apply.
type recordingWriter struct {
	header      http.Header
	status      int
	body        []byte
	wroteHeader bool
	// explicit is set only by a direct WriteHeader call, never by the
	// implied 200 on first Write or by terminal upstream publication.
	explicit bool
}

func newRecordingWriter() *recordingWriter {
	return &recordingWriter{
		header: make(http.Header),
	}
}

// Header returns the recorded response headers.
func (w *recordingWriter) Header() http.Header { return w.header }

// Write appends to the recorded body, implying status 200 on first write.
func (w *recordingWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.writeHeader(http.StatusOK, false)
	}

	w.body = append(w.body, b...)
	return len(b), nil
}

// WriteHeader records the status code. The first call wins.
func (w *recordingWriter) WriteHeader(statusCode int) {
	w.writeHeader(statusCode, true)
}

func (w *recordingWriter) writeHeader(statusCode int, explicit bool) {
	if w.wroteHeader {
		return
	}

	w.wroteHeader = true
	w.explicit = explicit
	w.status = statusCode
}

// setTerminal installs the upstream response without tripping the wrote
// flags user writes set, so the bridge can tell them apart.
func (w *recordingWriter) setTerminal(status int, upstream, stashed http.Header) {
	w.status = status

	h := make(http.Header, len(upstream)+len(stashed))
	for k, v := range upstream {
		h[k] = append([]string(nil), v...)
	}
	for k, v := range stashed {
		h[k] = append([]string(nil), v...)
	}
	w.header = h
}

// Status returns the recorded status code, or 0 if none was written.
func (w *recordingWriter) Status() int { return w.status }

// bridgeConfigKey carries the typed plugin config in a request context.
type bridgeConfigKey struct{}

// bridgeHooksKey carries bridge callbacks in a request context.
type bridgeHooksKey struct{}

// bridgeHooks are bridge callbacks a handler reaches through helpers.
// The handler goroutine never calls Envoy directly; the bridge applies
// reroutes and metric intents on callback threads.
type bridgeHooks struct {
	// attr reads a snapshotted attribute. Nil outside the bridge.
	attr func(name string) (string, bool)

	// reroute flags a route reload on proceed. Nil outside the bridge.
	reroute func()

	// log returns the request logger. Nil outside the bridge (discarded).
	log func() logr.Logger

	// sharedCache returns the filter-wide cache. Nil outside the bridge.
	sharedCache func() Cache

	// emitMetric queues a metric intent. Nil outside the bridge (dropped).
	emitMetric func(MetricOp)
}

// withBridgeConfig returns r carrying the typed plugin config.
func withBridgeConfig(r *http.Request, cfg any) *http.Request {
	return r.WithContext(stdcontext.WithValue(r.Context(), bridgeConfigKey{}, cfg))
}

// withBridgeHooks returns r carrying bridge callbacks.
func withBridgeHooks(r *http.Request, h bridgeHooks) *http.Request {
	return r.WithContext(stdcontext.WithValue(r.Context(), bridgeHooksKey{}, h))
}

// hooksOf returns the bridge callbacks on r, if any.
func hooksOf(r *http.Request) (bridgeHooks, bool) {
	h, ok := r.Context().Value(bridgeHooksKey{}).(bridgeHooks)
	return h, ok
}

// bridgeExchangeKey carries the live exchange in a request context.
type bridgeExchangeKey struct{}

// withBridgeExchange returns r carrying the live exchange.
func withBridgeExchange(r *http.Request, ex *bridgeExchange) *http.Request {
	return r.WithContext(stdcontext.WithValue(r.Context(), bridgeExchangeKey{}, ex))
}

// exchangeFrom returns the live exchange for r, if any.
func exchangeFrom(r *http.Request) (*bridgeExchange, bool) {
	ex, ok := r.Context().Value(bridgeExchangeKey{}).(*bridgeExchange)
	return ex, ok
}

// Terminal is the chain end: proceeding upstream means the response
// headers will arrive on W, which resumes G to run after-next code.
// It is stateless; the live exchange comes from the request. Chains
// terminate in a Terminal; calling it means "send upstream".
//
// Pass through the writer you received. Response bytes written into a
// different wrapper before next are not captured; write into w after
// next returns instead.
type Terminal struct{}

func (Terminal) ServeHTTP(w ResponseWriter, r *http.Request) {
	ex, ok := exchangeFrom(r)
	if !ok {
		return
	}

	ex.callNext(w, r)
}

func equalHeaderValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
