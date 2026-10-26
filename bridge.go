package gonvoy

import (
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"runtime"
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
	// brNeedSecret means the handler blocks waiting for a secret lookup.
	// W fulfills it and keeps waiting; it never ends the wait by itself.
	brNeedSecret
)

// secretRequest is one handler-goroutine secret lookup in flight.
type secretRequest struct {
	name  string
	reply chan secretResult
}

// secretResult is the fulfilled secret lookup.
type secretResult struct {
	value string
	found bool
}

// bridgeExchange carries one request through the coroutine bridge.
// The handler goroutine (G) touches only Go memory: snapshots, channels,
// the recording writer. Envoy C calls happen only on callback threads (W).
type bridgeExchange struct {
	handler httpHandler

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

	// earlyRespBody flags a pre-next response body touch. Reads before
	// next are a sequencing bug: the read fails with ErrRuntime and the
	// flag halts forwarding in callNext and applyDone, mirroring the
	// write-then-proceed guard.
	earlyRespBody atomic.Bool

	// nextWriter is the writer passed to next; after-next reads and writes
	// go through it. Set once by G before signaling proceed.
	nextWriter ResponseWriter

	failErr      error
	replacedBody []byte
	reroute      bool
	refreshRoute bool
	refreshDone  bool
	// liveRespHeader is the Envoy response map for in-place mutation.
	liveRespHeader api.HeaderMap

	// Upstream override, set by the handler and applied by W.
	overrideHost   string
	overrideStrict bool
	hasOverride    bool

	// drain flags the connection for drain after the request completes.
	// drainApplied guards the one-shot application on W.
	drain        bool
	drainApplied bool

	// TLS snapshot, taken on W at request headers. Never mutated after.
	tls   TLSConnection
	tlsOK bool

	// streamInfo caches the Envoy stream info for this request: one C call
	// per request instead of one per decision. May be nil in tests.
	streamInfo api.StreamInfo

	// Secret rendezvous. G stores one request at a time and blocks; W
	// fulfills it from inside awaitSignal and G resumes.
	secretMu     sync.Mutex
	secretReq    secretRequest
	secretLookup func(name string) (string, bool)
	// secretCache memoizes lookups within one request: repeated Secret()
	// calls for the same name skip the channel round-trip + cgo GetGenericSecret.
	secretCache map[string]secretResult

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

	// hooksWired records that hooks was built for this exchange. Pooled
	// exchanges keep their ex-capturing closures across reuses: every one
	// closes over the same exchange pointer, so only the stream-stable trio
	// (log, sharedCache, secretLookup) refreshes per use.
	hooksWired bool
}

// newBridgeExchange builds the exchange. Hooks may carry nil funcs when
// running outside the bridge (direct handler unit tests).
func newBridgeExchange(h httpHandler, req *http.Request, w *recordingWriter, hooks bridgeHooks) *bridgeExchange {
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

// bridgeExchangePool recycles exchanges across requests on the coroutine
// path. Release happens only in OnDestroy once the handler goroutine has
// exited (gDone), so no live state crosses requests.
var bridgeExchangePool sync.Pool

// acquireBridgeExchange returns a reset exchange bound to (h, req). Pooled
// exchanges keep their writer, readers, maps, channels, and ex-capturing
// hooks: reset drops contents, newExchange refills per-request values.
func acquireBridgeExchange(h httpHandler, req *http.Request) *bridgeExchange {
	if v := bridgeExchangePool.Get(); v != nil {
		ex := v.(*bridgeExchange)
		resetBridgeExchange(ex)
		ex.handler = h
		ex.req = req

		return ex
	}

	return newBridgeExchange(h, req, newRecordingWriter(), bridgeHooks{})
}

// releaseBridgeExchange returns ex to the pool after a clean completion. A
// handler still running at destroy (client abort) drops instead: its
// goroutine may still touch the exchange while unwinding.
func releaseBridgeExchange(ex *bridgeExchange) {
	if ex == nil || !ex.gDone.Load() {
		return
	}

	resetBridgeExchange(ex)
	bridgeExchangePool.Put(ex)
}

// resetBridgeExchange drops every per-request value so pooled state never
// leaks across requests. Anything that could outlive the stream (body bytes,
// TLS slices, live Envoy maps) releases; capacities stay for reuse.
// Channels drain; done/respReady renew (a closed channel cannot reset).
func resetBridgeExchange(ex *bridgeExchange) {
	resetRecordingWriter(ex.w)

	ex.req = nil
	// Snapshot maps clear but survive: newExchange and EncodeHeaders refill
	// them in place, reusing buckets instead of reallocating per request.
	if ex.reqSnapshot != nil {
		clear(ex.reqSnapshot)
	}
	if ex.respHeader != nil {
		clear(ex.respHeader)
	}
	if ex.preHeaders != nil {
		clear(ex.preHeaders)
	}
	ex.reqHeaderMap = nil
	ex.liveRespHeader = nil
	ex.streamInfo = nil
	ex.tls = TLSConnection{}
	ex.tlsOK = false

	ex.origHost = ""
	ex.origMethod = ""
	ex.origPath = ""

	ex.nextCalled.Store(false)
	ex.proceedUpstream.Store(false)
	ex.gDone.Store(false)
	ex.earlyRespBody.Store(false)
	ex.nextWriter = nil
	ex.failErr = nil
	ex.replacedBody = nil
	ex.reroute = false
	ex.refreshRoute = false
	ex.refreshDone = false
	ex.overrideHost = ""
	ex.overrideStrict = false
	ex.hasOverride = false
	ex.drain = false
	ex.drainApplied = false

	ex.secretReq = secretRequest{}
	if ex.secretCache != nil {
		clear(ex.secretCache)
	}

	for i := range ex.bodyChunks {
		ex.bodyChunks[i] = nil
	}
	ex.bodyChunks = ex.bodyChunks[:0]
	ex.bodyEnded = false
	for i := range ex.respChunks {
		ex.respChunks[i] = nil
	}
	ex.respChunks = ex.respChunks[:0]
	ex.respEnded = false
	ex.respCode = 0

	drainSignal(ex.bodyWake)
	drainSignal(ex.respWake)
drainSig:
	for {
		select {
		case <-ex.sigCh:
		default:
			break drainSig
		}
	}
	ex.respReady = make(chan struct{})
	ex.respReadyOnce = sync.Once{}
	ex.done = make(chan struct{})
	ex.doneOnce = sync.Once{}

	ex.metricOps = ex.metricOps[:0]
	if ex.attrs != nil {
		clear(ex.attrs)
	} else {
		ex.attrs = make(map[string]string)
	}
}

// drainSignal drops one pending wakeup, if any.
func drainSignal(ch chan struct{}) {
	select {
	case <-ch:
	default:
	}
}

// signal sends to W without blocking. Signals back up while W waits;
// a full channel means W already has what it needs.
// signal sends a handler-goroutine signal to the waiting W thread.
// Body requests are lossy hints: W already holds the stream and a pending
// wake is enough. terminal signals (proceed/done/failed) block until W
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
// A needSecret signal is fulfilled inline and the wait continues: the
// secret lookup runs on W, G gets its answer, and the stream still pauses
// until G's terminal signal arrives.
//
// Timeout callers return StopNoBuffer on data paths (nothing new to buffer;
// the bytes that triggered the wait are already held) and StopAndBuffer on
// header paths (headers must stay buffered until the decision lands).
// Both converge at the next callback through drainDecision.
func (ex *bridgeExchange) awaitSignal() (brSignal, bool) {
	// Fast path: handler already signaled (common for header-only filters).
	// Avoids the time.After timer alloc on every request.
	select {
	case s := <-ex.sigCh:
		if s == brNeedSecret {
			ex.fulfillSecret()
		} else {
			return s, true
		}
	case <-ex.done:
		return 0, false
	default:
	}

	for {
		timeout := time.After(time.Second)

		select {
		case s := <-ex.sigCh:
			if s == brNeedSecret {
				ex.fulfillSecret()
				continue
			}

			return s, true
		case <-ex.done:
			return 0, false
		case <-timeout:
			return 0, false
		}
	}
}

// fulfillSecret answers one pending secret lookup on W and lets the wait
// continue. Runs only on callback threads.
func (ex *bridgeExchange) fulfillSecret() {
	ex.secretMu.Lock()
	req := ex.secretReq
	ex.secretMu.Unlock()

	if req.reply == nil || ex.secretLookup == nil {
		return
	}

	v, ok := ex.secretLookup(req.name)
	req.reply <- secretResult{value: v, found: ok}
}

// requestSecret asks W for a named secret and blocks G until it arrives
// or the stream dies. Runs only on the handler goroutine. Results are
// memoized per exchange: repeated lookups skip the rendezvous + cgo.
func (ex *bridgeExchange) requestSecret(name string) (string, bool) {
	ex.secretMu.Lock()
	if res, ok := ex.secretCache[name]; ok {
		ex.secretMu.Unlock()
		return res.value, res.found
	}
	reply := make(chan secretResult, 1)
	ex.secretReq = secretRequest{name: name, reply: reply}
	ex.secretMu.Unlock()

	ex.signal(brNeedSecret)

	select {
	case res := <-reply:
		ex.secretMu.Lock()
		if ex.secretCache == nil {
			ex.secretCache = make(map[string]secretResult)
		}
		ex.secretCache[name] = res
		ex.secretMu.Unlock()
		return res.value, res.found
	case <-ex.done:
		return "", false
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

// poisonRespReader fails response body reads before next was called.
// Evaluating the Body field is harmless; the violation triggers on Read,
// the harmful act, so debug prints and nil checks never false-positive.
// It mirrors the write-then-proceed halt: the read logs, flags the
// exchange, and errors with ErrRuntime so returning it answers 500;
// proceeding anyway fails in callNext all the same.
type poisonRespReader struct {
	ex *bridgeExchange
}

// Close is a no-op; nothing was opened.
func (p *poisonRespReader) Close() error { return nil }

// Read records the phase violation and fails.
func (p *poisonRespReader) Read([]byte) (int, error) {
	return 0, p.ex.failEarlyRespBody()
}

// failEarlyRespBody records a pre-next response body touch and reports it
// with ErrRuntime so the error handler answers 500. The logger is resolved
// lazily so Response() views that never read Body pay no Log lookup.
func (ex *bridgeExchange) failEarlyRespBody() error {
	err := fmt.Errorf("response body is not available before next, %w", ErrRuntime)
	log := logr.Discard()
	if ex.hooks.log != nil {
		log = ex.hooks.log()
	}
	log.Error(err, "response body read before next; proceed upstream with next(c) first")
	ex.earlyRespBody.Store(true)

	return err
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

// handlerTaskQueue hands exchanges to pooled handler workers. Unbuffered: a
// send rendezvous with an idle worker, so saturation overflows instead of
// queueing latency behind busy workers.
var handlerTaskQueue = make(chan *bridgeExchange)

// numHandlerWorkers sizes the pool: twice the CPUs, at least 8. Fast tasks
// (deny, passthrough) recycle a worker in microseconds; parked next-paths
// hold one for milliseconds, so saturation overflows to one-off goroutines
// instead of hogging the pool.
var numHandlerWorkers = handlerWorkerCount()

func handlerWorkerCount() int {
	if n := 2 * runtime.NumCPU(); n >= 8 {
		return n
	}

	return 8
}

func init() {
	for i := 0; i < numHandlerWorkers; i++ {
		go handlerWorkerLoop()
	}
}

// handlerWorkerLoop runs exchange handlers until the process exits. Idle
// workers park on the queue with no CPU cost. Panics stay isolated per task
// inside runHandler's own recovery.
func handlerWorkerLoop() {
	for ex := range handlerTaskQueue {
		ex.runHandler()
	}
}

// submitHandlerTask runs ex's handler without spawning per request: an idle
// worker takes it, otherwise a one-off goroutine does, identical to the old
// go statement. W never blocks here.
func submitHandlerTask(ex *bridgeExchange) {
	select {
	case handlerTaskQueue <- ex:
	default:
		go ex.runHandler()
	}
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

	// A pre-next response body touch is a sequencing bug, like writing a
	// response and then proceeding. Fail instead of forwarding silently.
	if ex.earlyRespBody.Load() {
		ex.fail(fmt.Errorf("handler read response body before next, %w", ErrRuntime))
		return
	}

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
	// before next were already rejected above. The stash map survives pooled
	// reuses via refill.
	ex.preHeaders = refillHeader(ex.preHeaders, w.Header())
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
	for k, v := range ex.preHeaders {
		w.Header()[k] = v
	}
	for k, v := range ex.respHeader {
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

// refillHeader copies src into dst, reusing dst's buckets. A nil dst clones.
// Pooled exchanges refill snapshots instead of reallocating them per request.
func refillHeader(dst, src http.Header) http.Header {
	if dst == nil {
		return cloneHeader(src)
	}

	clear(dst)
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}

	return dst
}

func uint64ptr(v uint64, ok bool) *uint64 {
	if !ok {
		return nil
	}

	return &v
}

func uint16ptr(v uint16, ok bool) *uint16 {
	if !ok {
		return nil
	}

	return &v
}

func stringptr(v string, ok bool) *string {
	if !ok {
		return nil
	}

	return &v
}

// snapshotTLS copies an Envoy TLS connection into plain Go memory.
// Slices are deep-copied; it runs on W while the connection info is live.
func snapshotTLS(sc api.SslConnection) TLSConnection {
	return TLSConnection{
		PeerCertificatePresented:    sc.PeerCertificatePresented(),
		PeerCertificateValidated:    sc.PeerCertificateValidated(),
		Sha256PeerCertificateDigest: sc.Sha256PeerCertificateDigest(),
		SerialNumberPeerCertificate: sc.SerialNumberPeerCertificate(),
		SubjectPeerCertificate:      sc.SubjectPeerCertificate(),
		IssuerPeerCertificate:       sc.IssuerPeerCertificate(),
		SubjectLocalCertificate:     sc.SubjectLocalCertificate(),
		URISANsPeerCertificate:      append([]string(nil), sc.UriSanPeerCertificate()...),
		URISANsLocalCertificate:     append([]string(nil), sc.UriSanLocalCertificate()...),
		DNSSANsPeerCertificate:      append([]string(nil), sc.DnsSansPeerCertificate()...),
		DNSSANsLocalCertificate:     append([]string(nil), sc.DnsSansLocalCertificate()...),
		ValidFromPeerCertificate: func() *uint64 {
			v, ok := sc.ValidFromPeerCertificate()
			return uint64ptr(v, ok)
		}(),
		ExpirationPeerCertificate: func() *uint64 {
			v, ok := sc.ExpirationPeerCertificate()
			return uint64ptr(v, ok)
		}(),
		TLSVersion:  sc.TlsVersion(),
		Ciphersuite: sc.CiphersuiteString(),
		CiphersuiteID: func() *uint16 {
			v, ok := sc.CiphersuiteId()
			return uint16ptr(v, ok)
		}(),
		SessionID: func() *string {
			v, ok := sc.SessionId()
			return stringptr(v, ok)
		}(),
		PEMPeerCertificate:      sc.UrlEncodedPemEncodedPeerCertificate(),
		PEMPeerCertificateChain: sc.UrlEncodedPemEncodedPeerCertificateChain(),
	}
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

// resetRecordingWriter clears w for reuse: header entries dropped, status and
// flags zeroed, body kept at zero length with its capacity.
func resetRecordingWriter(w *recordingWriter) {
	clear(w.header)
	w.status = 0
	w.body = w.body[:0]
	w.wroteHeader = false
	w.explicit = false
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
// flags user writes set, so the bridge can tell them apart. Upstream wins
// on key conflicts: pre-next writes staged below only fill keys the
// upstream did not set, and post-next writes win through the header diff.
func (w *recordingWriter) setTerminal(status int, upstream, stashed http.Header) {
	w.status = status

	h := make(http.Header, len(upstream)+len(stashed))
	for k, v := range stashed {
		h[k] = append([]string(nil), v...)
	}
	for k, v := range upstream {
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

	// overrideUpstream pins the request to one upstream host.
	// Nil outside the bridge.
	overrideUpstream func(host string, strict bool)

	// drain flags the connection for drain after the request completes.
	// Nil outside the bridge.
	drain func()

	// secret fetches a named secret via a callback-thread rendezvous.
	// Nil outside the bridge.
	secret func(name string) (string, bool)

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
	if v, ok := r.Context().Value(bridgeRequestKey{}).(bridgeRequestValues); ok {
		return v.hooks, true
	}
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
	if v, ok := r.Context().Value(bridgeRequestKey{}).(bridgeRequestValues); ok && v.ex != nil {
		return v.ex, true
	}
	ex, ok := r.Context().Value(bridgeExchangeKey{}).(*bridgeExchange)
	return ex, ok
}

// bridgeRequestKey carries all per-request bridge values in one context node.
// newExchange stores config+hooks+exchange once instead of three nested
// WithContext layers (3 request copies + 3 nodes per request).
type bridgeRequestKey struct{}

// bridgeRequestValues is the single per-request context payload.
type bridgeRequestValues struct {
	cfg   any
	hooks bridgeHooks
	ex    *bridgeExchange
}

// withBridgeValues returns r carrying cfg+hooks+ex in one context node.
func withBridgeValues(r *http.Request, cfg any, h bridgeHooks, ex *bridgeExchange) *http.Request {
	return r.WithContext(stdcontext.WithValue(r.Context(), bridgeRequestKey{}, bridgeRequestValues{cfg: cfg, hooks: h, ex: ex}))
}

// terminal is the internal chain end: proceeding upstream means the
// response headers will arrive on W, which resumes G to run after-next
// code. It is stateless; the live exchange comes from the request. Chains
// built by Use terminate here; calling it means "send upstream". It stays
// unexported so user code proceeds through the Middleware chain instead of
// reaching the upstream hop directly.
//
// Pass through the writer you received. Response bytes written into a
// different wrapper before next are not captured; write into w after
// next returns instead.
type terminal struct{}

func (terminal) ServeHTTP(w ResponseWriter, r *http.Request) {
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
