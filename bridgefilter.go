package gonvoy

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/ardikabs/gonvoy/pkg/util"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/go-logr/logr"
)

// bridgeFilter is an api.StreamFilter driving one net/http middleware chain
// per request. It owns a *context for Envoy I/O and runs user code on a
// separate goroutine against snapshots. All C calls happen on callback
// threads; the goroutine touches only Go memory plus Log.
type bridgeFilter struct {
	ctx *context
	h   httpHandler

	exchange *bridgeExchange

	errorHandler func(bridgeContext, error) api.StatusType
	sendReply    func(ex *bridgeExchange, status int, body []byte) api.StatusType

	// Shared per-stream closures: hoisted from per-request allocs.
	// logFn/cacheFn/secretLookupFn close over ctx only, identical for every
	// request on this stream. Per-request hooks still close over ex.
	sharedLog          func() logr.Logger
	sharedCache        func() Cache
	sharedSecretLookup func(name string) (string, bool)
}

var NoOpHttpFilter = &api.PassThroughStreamFilter{}

// NewHandlerChainFactory builds a StreamFilterFactory serving h for every request.
// The handler runs once per request; returning without writing or calling
// next continues upstream. Plugin config, body access, and metrics come
// from the parsed route config, replacing the retired phase-based path.
func NewHandlerChainFactory(h Handler) api.StreamFilterFactory {
	if util.IsNil(h) {
		panic("handlerChainFactory: handler shouldn't be nil")
	}

	return func(cfg interface{}, cb api.FilterCallbackHandler) api.StreamFilter {
		config, ok := cfg.(*internalConfig)
		if !ok {
			panic(fmt.Sprintf("handlerChainFactory: unexpected config type '%T', expecting '%T'", cfg, config))
		}

		// Logger builds lazily on first Log: the hot path pays nothing here.
		ctx, err := newBridgeContext(cb, contextOptions{
			config: config,
		})
		if err != nil {
			newLogger(cb).Error(err, "failed to initialize context for filter, ignoring filter ...")
			return NoOpHttpFilter
		}

		return newBridgeFilter(mustCastToContext(ctx), h)
	}
}

func newBridgeFilter(c *context, h httpHandler) *bridgeFilter {
	f := &bridgeFilter{
		ctx:          c,
		h:            h,
		errorHandler: func(c bridgeContext, err error) api.StatusType { return defaultErrorHandler(c, err) },
	}
	f.sendReply = f.defaultSendReply
	// Hoist stream-stable closures once: identical for every request.
	f.sharedLog = func() logr.Logger { return c.Log() }
	f.sharedCache = func() Cache { return c.GetCache() }
	f.sharedSecretLookup = func(name string) (string, bool) {
		return c.cb.SecretManager().GetGenericSecret(name)
	}
	return f
}

// defaultSendReply emits a local reply through the context.
func (f *bridgeFilter) defaultSendReply(ex *bridgeExchange, status int, body []byte) api.StatusType {
	c := f.ctx

	if status == 0 {
		status = http.StatusOK
	}

	f.flushMetrics(ex)

	if err := c.SendResponse(status, string(body), LocalReplyWithHTTPHeaders(ex.w.Header())); err != nil {
		c.Log().Error(err, "failed to send local reply")
	}

	return api.LocalReply
}

// EncodeHeaders snapshots the upstream response and resumes the handler.
// It always unblocks a parked next(); if the handler has not proceeded,
// the response streams through unmodified.
func (f *bridgeFilter) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	ex := f.exchange
	if ex == nil {
		return api.Continue
	}

	// Clean passthrough: the handler returned without proceeding and left no
	// late signal. Nobody waits for this response and nobody reads it, so
	// skip the snapshot, the reader setup, the refresh, and the flush: only
	// pre-next response header sets still apply. A drained late signal takes
	// the full path instead.
	if ex.gDone.Load() && !ex.nextCalled.Load() {
		if sig, ok := ex.drainDecision(); ok {
			return f.applyResponseDecision(ex, header, nil, endStream, sig)
		}

		if len(ex.w.Header()) > 0 {
			applyHeaderSets(header, ex.w.Header())
		}

		return api.Continue
	}

	c := f.ctx
	c.pcb = c.cb.EncoderFilterCallbacks()
	c.LoadResponseHeaders(header)

	code, _ := header.Status()
	ex.respCode = code
	ex.respHeader = refillRequestHeaders(header, ex.respHeader)
	ex.liveRespHeader = header
	// Reuse the respReader built in newExchange: only the gate flips per
	// phase. Encode used to allocate a second reader per request.
	respEnabled := c.IsResponseBodyReadable() || c.IsResponseBodyWritable()
	if ex.respReader == nil {
		ex.respReader = &gatedRespReader{ex: ex, enabled: respEnabled}
	} else {
		ex.respReader.enabled = respEnabled
	}
	if c.IsResponseBodyWritable() {
		_ = shouldOmitContentLengthOnResponse(c, header)
	}
	ex.feedRespBody(nil, endStream)
	ex.respReadyOnce.Do(func() { close(ex.respReady) })

	f.refreshAttrs(ex, ex.req, true)
	f.flushMetrics(ex)

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopAndBuffer
	}

	return f.applyResponseDecision(ex, header, nil, endStream, sig)
}

// EncodeData feeds buffered response bytes and applies the handler's next
// decision. Without response body access the body streams through untouched.
func (f *bridgeFilter) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	ex := f.exchange
	if ex == nil {
		return api.Continue
	}

	f.flushMetrics(ex)

	if ex.gDone.Load() {
		if sig, ok := ex.drainDecision(); ok {
			return f.applyResponseDecision(ex, nil, buffer, endStream, sig)
		}

		return f.applyPendingBody(ex, buffer, endStream)
	}

	if f.bufferResponseBody() {
		ex.feedRespBody(buffer.Bytes(), endStream)
	}

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopNoBuffer
	}

	return f.applyResponseDecision(ex, nil, buffer, endStream, sig)
}

// bufferResponseBody reports whether upstream response bytes are buffered
// for the handler to read or replace.
func (f *bridgeFilter) bufferResponseBody() bool {
	return f.ctx.IsResponseBodyReadable() || f.ctx.IsResponseBodyWritable()
}

// applyPendingBody applies a stashed after-next replacement once the
// upstream body is complete. Body-only writes never survive past the
// response they were written into.
func (f *bridgeFilter) applyPendingBody(ex *bridgeExchange, buffer api.BufferInstance, endStream bool) api.StatusType {
	w := ex.w

	if len(w.body) > 0 && !w.explicit && f.bufferResponseBody() {
		if !f.ctx.IsResponseBodyWritable() {
			return f.errorHandler(f.ctx, fmt.Errorf("response body is not writable, %w", ErrOperationNotPermitted))
		}

		if !endStream || buffer == nil {
			return api.StopAndBuffer
		}

		body := append([]byte(nil), w.body...)
		w.body = nil

		if err := buffer.Set(body); err != nil {
			return f.errorHandler(f.ctx, err)
		}
	}

	return api.Continue
}

// applyResponseDecision applies G's after-next signal on the W thread.
func (f *bridgeFilter) applyResponseDecision(ex *bridgeExchange, header api.ResponseHeaderMap, buffer api.BufferInstance, endStream bool, sig brSignal) api.StatusType {
	c := f.ctx
	f.flushMetrics(ex)
	f.applyDrain(ex)

	switch sig {
	case brNeedBody:
		return api.StopAndBuffer
	case brFailed:
		return f.errorHandler(c, ex.failErr)
	case brProceed:
		return api.Continue
	case brDone:
		return f.applyAfterNext(ex, header, buffer, endStream)
	}

	return api.Continue
}

// applyAfterNext handles a returned handler after next was called.
// Any write becomes a full local reply; otherwise the upstream response
// flows through with the handler's header mutations applied.
func (f *bridgeFilter) applyAfterNext(ex *bridgeExchange, header api.ResponseHeaderMap, buffer api.BufferInstance, endStream bool) api.StatusType {
	c := f.ctx
	w := ex.w

	if !w.wroteHeader && len(w.body) == 0 {
		if header != nil {
			if ex.nextCalled.Load() && ex.nextWriter != nil {
				applyHeaderDiff(header, ex.respHeader, ex.nextWriter.Header())
			} else {
				applyHeaderSets(header, w.Header())
			}
		}

		return api.Continue
	}

	// An explicit status always wins a full local reply. A body-only write
	// replaces the buffered upstream body when there is one to replace
	// into; with nowhere to put it, it becomes a full reply instead.
	if w.explicit || !f.bufferResponseBody() || (buffer == nil && endStream) {
		status := w.status
		if status == 0 {
			status = ex.respCode
		}

		return f.sendReply(ex, status, w.body)
	}

	if !c.IsResponseBodyWritable() {
		return f.errorHandler(c, fmt.Errorf("response body is not writable, %w", ErrOperationNotPermitted))
	}

	if buffer != nil && endStream {
		if err := buffer.Set(w.body); err != nil {
			return f.errorHandler(c, err)
		}

		w.body = nil
		return api.Continue
	}

	return api.StopAndBuffer
}

// applyHeaderSets adds recorded headers the live map does not already
// carry. Keys present on the live (upstream) map are left untouched:
// upstream wins on conflict, and only post-next writes override it
// through the header diff. Used when there is no upstream snapshot to
// diff against.
func applyHeaderSets(live api.HeaderMap, current http.Header) {
	for k, v := range current {
		if _, ok := live.Get(k); ok {
			continue
		}
		live.Del(k)
		for _, vv := range v {
			live.Add(k, vv)
		}
	}
}

// DecodeTrailers treats trailers as the end of the request body.
// DecodeTrailers treats trailers as the end of the request body, feeding
// EOF and applying the handler's decision like any other data callback.
func (f *bridgeFilter) DecodeTrailers(api.RequestTrailerMap) api.StatusType {
	ex := f.exchange
	if ex == nil {
		return api.Continue
	}

	if ex.gDone.Load() {
		if sig, ok := ex.drainDecision(); ok {
			return f.applyRequestDecision(ex, nil, nil, true, sig)
		}

		return api.Continue
	}

	ex.feedBody(nil, true)

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopNoBuffer
	}

	return f.applyRequestDecision(ex, nil, nil, true, sig)
}

// EncodeTrailers treats trailers as the end of the response body.
func (f *bridgeFilter) EncodeTrailers(api.ResponseTrailerMap) api.StatusType {
	ex := f.exchange
	if ex == nil {
		return api.Continue
	}

	if ex.gDone.Load() {
		if sig, ok := ex.drainDecision(); ok {
			return f.applyResponseDecision(ex, nil, nil, true, sig)
		}

		return api.Continue
	}

	if f.bufferResponseBody() {
		ex.feedRespBody(nil, true)
	}

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopNoBuffer
	}

	return f.applyResponseDecision(ex, nil, nil, true, sig)
}

// OnDestroy unblocks the handler goroutine and drops queued work. Cleanly
// finished exchanges return to the pool; aborted ones drop.
func (f *bridgeFilter) OnDestroy(api.DestroyReason) {
	if ex := f.exchange; ex != nil {
		ex.destroy()
		releaseBridgeExchange(ex)
	}

	f.exchange = nil
}

// OnLog flushes metrics at access-log time.
func (f *bridgeFilter) OnLog(api.RequestHeaderMap, api.RequestTrailerMap, api.ResponseHeaderMap, api.ResponseTrailerMap) {
	if ex := f.exchange; ex != nil {
		f.flushMetrics(ex)
	}
}

// OnLogDownstreamStart is a no-op for the bridge.
func (f *bridgeFilter) OnLogDownstreamStart(api.RequestHeaderMap) {}

// OnLogDownstreamPeriodic is a no-op for the bridge.
func (f *bridgeFilter) OnLogDownstreamPeriodic(api.RequestHeaderMap, api.RequestTrailerMap, api.ResponseHeaderMap, api.ResponseTrailerMap) {
}

// OnStreamComplete flushes metrics at stream completion.
func (f *bridgeFilter) OnStreamComplete() {
	if ex := f.exchange; ex != nil {
		f.flushMetrics(ex)
	}
}

// DecodeHeaders snapshots the request, starts the handler, and applies its
// first decision. It never blocks on future data: a handler that blocks on
// the body gets StopAndBuffer, everything else is decided immediately.
func (f *bridgeFilter) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	c := f.ctx
	c.pcb = c.cb.DecoderFilterCallbacks()

	ex := f.newExchange(header, endStream)
	f.exchange = ex
	submitHandlerTask(ex)

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopAndBuffer
	}

	return f.applyRequestDecision(ex, header, nil, endStream, sig)
}

// DecodeData feeds body bytes and applies the handler's next decision.
func (f *bridgeFilter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	ex := f.exchange
	if ex == nil || ex.proceedUpstream.Load() {
		return f.streamRequestBody(ex, buffer, endStream)
	}

	if ex.gDone.Load() {
		if sig, ok := ex.drainDecision(); ok {
			return f.applyRequestDecision(ex, nil, buffer, endStream, sig)
		}

		return api.Continue
	}

	ex.feedBody(buffer.Bytes(), endStream)

	sig, ok := ex.awaitSignal()
	if !ok {
		return api.StopNoBuffer
	}

	return f.applyRequestDecision(ex, nil, buffer, endStream, sig)
}

// drainDecision consumes one terminal signal left behind by an already
// finished handler, typically after a wait timed out. Stale body requests
// are ignored since G will never act on them. Returns false when there is
// nothing to apply.
func (ex *bridgeExchange) drainDecision() (brSignal, bool) {
	for {
		select {
		case sig := <-ex.sigCh:
			if sig == brNeedBody {
				continue
			}

			return sig, true
		default:
			return 0, false
		}
	}
}

// applyRequestDecision applies G's signal on the W thread. It is called with
// fresh data in hand, so G always has something to decide on except when it
// asked for more body, in which case more is guaranteed to arrive.
func (f *bridgeFilter) applyRequestDecision(ex *bridgeExchange, header api.RequestHeaderMap, buffer api.BufferInstance, endStream bool, sig brSignal) api.StatusType {
	f.flushMetrics(ex)
	f.applyDrain(ex)

	switch sig {
	case brNeedBody:
		return api.StopAndBuffer
	case brFailed:
		return f.errorHandler(f.ctx, ex.failErr)
	case brProceed:
		return f.applyProceed(ex, header, buffer, endStream)
	case brDone:
		return f.applyDone(ex)
	}

	return api.Continue
}

// propagateRequestLine applies the exchange's mutated request-line fields to
// the live Envoy map.
func (f *bridgeFilter) propagateRequestLine(ex *bridgeExchange, live api.RequestHeaderMap) {
	propagateRequestLine(ex.req, ex.origHost, ex.origMethod, ex.origPath, live)
}

// propagateRequestLine applies handler-mutated request-line fields (path,
// host, method) to the live Envoy map. Plain header diffing cannot do this:
// :path, :authority, and :method are pseudo-headers outside http.Header.
func propagateRequestLine(r *http.Request, origHost, origMethod, origPath string, live api.RequestHeaderMap) {
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}

	if path != origPath {
		live.SetPath(path)
	}

	if r.Host != origHost {
		live.SetHost(r.Host)
	}

	if r.Method != origMethod {
		live.SetMethod(r.Method)
	}
}

// applyProceed continues upstream: header diff, reroute, body replacement.
func (f *bridgeFilter) applyProceed(ex *bridgeExchange, header api.RequestHeaderMap, buffer api.BufferInstance, endStream bool) api.StatusType {
	c := f.ctx

	if err := f.applyRequestMutations(ex, header); err != nil {
		return f.errorHandler(c, err)
	}

	if len(ex.replacedBody) > 0 {
		if !c.IsRequestBodyWritable() {
			return f.errorHandler(c, fmt.Errorf("request body is not writable, %w", ErrOperationNotPermitted))
		}

		if buffer != nil && endStream {
			if err := buffer.Set(ex.replacedBody); err != nil {
				return f.errorHandler(c, err)
			}
			ex.replacedBody = nil
		} else if buffer == nil && endStream {
			c.cb.DecoderFilterCallbacks().AddData(ex.replacedBody, false)
			ex.replacedBody = nil
		}
	}

	return api.Continue
}

// requestMutated reports whether the handler changed routing-relevant
// request state since DecodeHeaders.
func (f *bridgeFilter) requestMutated(ex *bridgeExchange) bool {
	return headersMutated(ex.req, ex.reqSnapshot, ex.origHost, ex.origMethod, ex.origPath)
}

// headersMutated reports whether r drifted from its DecodeHeaders snapshot in
// routing-relevant state: host, method, path, or any header addition, change,
// or removal. Pure Go, shared by the coroutine bridge and the header-only path.
func headersMutated(r *http.Request, snapshot http.Header, origHost, origMethod, origPath string) bool {
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}

	if r.Host != origHost || r.Method != origMethod || path != origPath {
		return true
	}

	current := r.Header
	for k, v := range current {
		if old, ok := snapshot[k]; !ok || !equalHeaderValues(old, v) {
			return true
		}
	}

	for k := range snapshot {
		if _, ok := current[k]; !ok {
			return true
		}
	}

	return false
}

// applyDone handles a returned handler: local reply or continue.
func (f *bridgeFilter) applyDone(ex *bridgeExchange) api.StatusType {
	w := ex.w

	// A pre-next response body touch that never proceeded still halts:
	// the read already failed, and passing through would hide the bug.
	if ex.earlyRespBody.Load() {
		return f.errorHandler(f.ctx, fmt.Errorf("handler read response body without next, %w", ErrRuntime))
	}

	if !w.wroteHeader && len(w.body) == 0 {
		// No answer written: mutations still apply, mirroring immediate
		// setters, and the stream continues upstream.
		if err := f.applyRequestMutations(ex, nil); err != nil {
			return f.errorHandler(f.ctx, err)
		}

		return api.Continue
	}

	return f.sendReply(ex, w.status, w.body)
}

// applyRequestMutations pushes handler-side request mutations (headers,
// path/host/method, upstream override, route reload) to the live Envoy
// map. It runs on every request-completion path, mirroring the old
// immediate setters. An upstream-override failure is the only error:
// Envoy rejects hosts that are not an IP address with a port.
func (f *bridgeFilter) applyRequestMutations(ex *bridgeExchange, header api.RequestHeaderMap) error {
	c := f.ctx

	var live api.HeaderMap = header
	if live == nil {
		live = ex.reqHeaderMap
	}

	if live != nil {
		applyHeaderDiff(live, ex.reqSnapshot, ex.req.Header)
	}

	if live != nil && c.IsRequestBodyWritable() {
		// Mirror the phase path: a writable request body implies chunked
		// encoding. Without this Envoy forwards the stale Content-Length
		// and the upstream reads a truncated replacement.
		shouldOmitContentLengthOnRequest(c, live)
	}

	if reqMap, ok := live.(api.RequestHeaderMap); ok {
		f.propagateRequestLine(ex, reqMap)
	}

	if ex.hasOverride {
		if err := c.cb.DecoderFilterCallbacks().SetUpstreamOverrideHost(ex.overrideHost, ex.overrideStrict); err != nil {
			return fmt.Errorf("failed to override upstream host, %w", err)
		}
	}

	if ex.refreshRoute && !ex.refreshDone {
		ex.refreshDone = true
		c.cb.RefreshRouteCache()
	} else if ex.reroute || (c.autoReloadRoute && f.requestMutated(ex)) {
		c.ReloadRoute()
	}

	return nil
}

// applyDrain marks the connection for drain after the current request
// completes. It runs on every decision point so a flag set at any phase
// takes effect exactly once. A nil stream info (unit tests) still consumes
// the flag without calling Envoy.
func (f *bridgeFilter) applyDrain(ex *bridgeExchange) {
	if ex.drain && !ex.drainApplied {
		ex.drainApplied = true
		if ex.streamInfo != nil {
			ex.streamInfo.DrainConnectionUponCompletion()
		}
	}
}

// flushMetrics applies queued metric intents in order. Runs on W only.
// Empty flushes are skipped without touching Metrics(): most requests queue
// nothing, but flush runs on every decision point plus OnLog/OnStreamComplete.
func (f *bridgeFilter) flushMetrics(ex *bridgeExchange) {
	ex.metricMu.Lock()
	if len(ex.metricOps) == 0 {
		ex.metricMu.Unlock()
		return
	}
	ops := ex.metricOps
	ex.metricOps = nil
	ex.metricMu.Unlock()

	m := f.ctx.Metrics()

	for _, op := range ops {
		switch op.Kind {
		case MetricCounterAdd:
			m.Counter(op.Name, op.Labels...).Increment(op.Delta)
		case MetricGaugeAdd:
			m.Gauge(op.Name, op.Labels...).Increment(op.Delta)
		case MetricGaugeSet:
			m.Gauge(op.Name, op.Labels...).Record(op.Value)
		}
	}
}

// refreshAttrs snapshots Envoy attributes for Attr lookups. request.* come
// from the live request (no cgo) and always land; response.code is
// synthesized. Every other key fetches via GetProperty only when declared
// through EnableEnvoyAttributes. request.* never change after decode.
func (f *bridgeFilter) refreshAttrs(ex *bridgeExchange, r *http.Request, response bool) {
	if !response {
		ex.attrMu.Lock()
		if r != nil {
			if r.Host != "" {
				if ex.attrs == nil {
					ex.attrs = make(map[string]string)
				}
				ex.attrs["request.host"] = r.Host
			}
			if r.Method != "" {
				if ex.attrs == nil {
					ex.attrs = make(map[string]string)
				}
				ex.attrs["request.method"] = r.Method
			}
			if r.URL != nil && r.URL.Path != "" {
				if ex.attrs == nil {
					ex.attrs = make(map[string]string)
				}
				ex.attrs["request.path"] = r.URL.Path
			}
		}
		ex.attrMu.Unlock()

		f.refreshDeclaredAttrs(ex)

		return
	}

	if ex.respCode != 0 {
		ex.attrMu.Lock()
		if ex.attrs == nil {
			ex.attrs = make(map[string]string)
		}
		ex.attrs["response.code"] = itoa(ex.respCode)
		ex.attrMu.Unlock()
	}

	f.refreshDeclaredAttrs(ex)
}

// refreshDeclaredAttrs fetches exactly the declared attribute names. Nil
// config (hand-built contexts in tests) or an empty declaration fetches
// nothing: only the free request fields and the synthesized response code
// resolve then.
func (f *bridgeFilter) refreshDeclaredAttrs(ex *bridgeExchange) {
	c := f.ctx

	var fetch []string
	if c.attrFetch != nil {
		fetch = c.attrFetch.fetch
	}

	for _, key := range fetch {
		if v, err := c.GetProperty(key, ""); err == nil && v != "" {
			ex.attrMu.Lock()
			if ex.attrs == nil {
				ex.attrs = make(map[string]string)
			}
			ex.attrs[key] = v
			ex.attrMu.Unlock()
		}
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// warnUndeclaredTLS logs the one-time migration warning when a downstream
// TLS connection is present but snapshotting is off. Runs on W only, at most
// once per filter registration. Plaintext (nil connection) stays silent.
func warnUndeclaredTLS(c *context, ex *bridgeExchange) {
	cfg := ex.attrFetch
	if cfg == nil {
		return
	}

	cfg.warn.Do(func() {
		c.Log().Info("TLS connection present without a snapshot; declare it with EnableDownstreamTLS")
	})
}

// newExchange builds the per-request exchange: snapshots, hooks, reader.
func (f *bridgeFilter) newExchange(header api.RequestHeaderMap, endStream bool) *bridgeExchange {
	c := f.ctx
	c.LoadRequestHeaders(header)

	// Single-materialization: LoadRequestHeaders already walked the C headers
	// once into c.httpReq.Header. Refill the snapshot Go-side instead of
	// ranging C again. Saves one cgo callback per header on every request.
	r := c.httpReq

	// Pooled exchanges arrive reset: writer, readers, maps, channels, and
	// ex-capturing hooks stay valid (same exchange pointer). Fresh exchanges
	// allocate each once, then join the pool after a clean destroy.
	ex := acquireBridgeExchange(f.h, r)

	// Reuse the gated readers: only the gate flips per request.
	reqBodyEnabled := c.IsRequestBodyReadable() || c.IsRequestBodyWritable()
	if ex.gatedBody == nil {
		ex.gatedBody = &gatedBodyReader{ex: ex, enabled: reqBodyEnabled}
	} else {
		ex.gatedBody.enabled = reqBodyEnabled
	}
	r.Body = ex.gatedBody
	respBodyEnabled := c.IsResponseBodyReadable() || c.IsResponseBodyWritable()
	if ex.respReader == nil {
		ex.respReader = &gatedRespReader{ex: ex, enabled: respBodyEnabled}
	} else {
		ex.respReader.enabled = respBodyEnabled
	}
	ex.reqSnapshot = refillHeader(ex.reqSnapshot, r.Header)
	ex.reqHeaderMap = header
	ex.origHost = r.Host
	ex.origMethod = r.Method
	if r.URL != nil {
		ex.origPath = r.URL.Path
	}

	// The prefetch config crosses pooled reuses like the log trio below:
	// plain pointer assignment, no allocation.
	ex.attrFetch = c.attrFetch

	if si := c.cb.StreamInfo(); si != nil {
		ex.streamInfo = si
		if sc := si.DownstreamSslConnection(); sc != nil {
			if c.enableDownstreamTLS {
				ex.tls = snapshotTLS(sc)
				ex.tlsOK = true
			} else {
				// TLS present but undeclared: migration warning, no
				// 19-getter snapshot. Plaintext (nil conn) stays silent.
				warnUndeclaredTLS(c, ex)
			}
		}
	}

	// Reuse stream-stable closures: no per-request alloc for log/cache/secret lookup.
	ex.secretLookup = f.sharedSecretLookup
	if ex.secretLookup == nil {
		ex.secretLookup = func(name string) (string, bool) {
			return c.cb.SecretManager().GetGenericSecret(name)
		}
	}
	logFn := f.sharedLog
	if logFn == nil {
		logFn = func() logr.Logger { return c.Log() }
	}
	cacheFn := f.sharedCache
	if cacheFn == nil {
		cacheFn = func() Cache { return c.GetCache() }
	}

	if !ex.hooksWired {
		// First use: every ex-capturing closure closes over this same
		// exchange pointer, so pooled reuses keep them as-is.
		ex.hooks = bridgeHooks{
			attr: func(name string) (string, bool) {
				ex.attrMu.Lock()
				v, ok := ex.attrs[name]
				ex.attrMu.Unlock()
				if !ok && cgoBackedAttr(name) {
					ex.warnUndeclaredAttr(name)
				}
				return v, ok
			},
			reroute: func() { ex.reroute = true },
			overrideUpstream: func(host string, strict bool) {
				ex.overrideHost, ex.overrideStrict, ex.hasOverride = host, strict, true
			},
			drain: func() { ex.drain = true },
			secret: func(name string) (string, bool) {
				return ex.requestSecret(name)
			},
			log:         logFn,
			sharedCache: cacheFn,
			emitMetric: func(op MetricOp) {
				ex.metricMu.Lock()
				ex.metricOps = append(ex.metricOps, op)
				ex.metricMu.Unlock()
			},
		}
		ex.hooksWired = true
	} else {
		// Pooled reuse: only the stream-stable trio may belong to another
		// stream's context. The rest still close over this exchange.
		ex.hooks.log = logFn
		ex.hooks.sharedCache = cacheFn
	}

	// Single context node: 1 request copy + 1 node instead of 3+3.
	r = withBridgeValues(r, c.filterConfig, ex.hooks, ex)
	ex.req = r

	f.refreshAttrs(ex, r, false)

	// Record stream end without waking: no reader can wait yet (the handler
	// submits after this), so feedBody's ping would only leave a stale token
	// for the first read to burn.
	ex.bodyMu.Lock()
	ex.bodyEnded = endStream
	ex.bodyMu.Unlock()

	return ex
}

// streamRequestBody forwards a body that is already proceeding upstream,
// replacing it at the end when the handler supplied one.
func (f *bridgeFilter) streamRequestBody(ex *bridgeExchange, buffer api.BufferInstance, endStream bool) api.StatusType {
	if ex != nil && len(ex.replacedBody) > 0 {
		if !f.ctx.IsRequestBodyWritable() {
			ex.replacedBody = nil
			return f.errorHandler(f.ctx, fmt.Errorf("request body is not writable, %w", ErrOperationNotPermitted))
		}
		if endStream {
			body := ex.replacedBody
			ex.replacedBody = nil

			if err := buffer.Set(body); err != nil {
				return f.errorHandler(f.ctx, err)
			}

			return api.Continue
		}

		return api.StopAndBuffer
	}

	return api.Continue
}

// snapshotRequestHeaders copies the live Envoy headers for diffing later.
func snapshotRequestHeaders(header api.HeaderMap) http.Header {
	snap := make(http.Header)

	header.Range(func(key, value string) bool {
		snap.Add(key, value)
		return true
	})

	return snap
}

// refillRequestHeaders copies the live Envoy headers into dst, reusing its
// buckets. A nil dst snapshots fresh. Pooled exchanges refill the response
// snapshot instead of reallocating it per response.
func refillRequestHeaders(header api.HeaderMap, dst http.Header) http.Header {
	if dst == nil {
		return snapshotRequestHeaders(header)
	}

	clear(dst)
	header.Range(func(key, value string) bool {
		dst.Add(key, value)
		return true
	})

	return dst
}

// applyHeaderDiff writes snapshot-to-current differences to the live map.
func applyHeaderDiff(live api.HeaderMap, snapshot, current http.Header) {
	for k, v := range current {
		old, ok := snapshot[k]
		if !ok || !equalHeaderValues(old, v) {
			live.Del(k)
			for _, vv := range v {
				live.Add(k, vv)
			}
		}
	}

	for k := range snapshot {
		if _, ok := current[k]; !ok {
			live.Del(k)
		}
	}
}
