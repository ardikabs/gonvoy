package gonvoy

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeDecoderCallbacks struct {
	api.DecoderFilterCallbacks
}

type fakeEncoderCallbacks struct {
	api.EncoderFilterCallbacks
}

func fakeBridgeHeaders(t *testing.T, contentType string) *mock_envoy.RequestHeaderMap {
	t.Helper()

	m := mock_envoy.NewRequestHeaderMap(t)
	m.EXPECT().Method().Return(http.MethodGet)
	m.EXPECT().Host().Return("example.com")
	m.EXPECT().Path().Return("/")
	m.EXPECT().Range(mock.Anything).Return().Run(func(f func(string, string) bool) {
		if contentType != "" {
			f(HeaderContentType, contentType)
		}
		f("X-Test", "1")
	})

	return m
}

func fakeBridgeContext(t *testing.T, req, resp string) *context {
	t.Helper()

	allowReqRead := req == "read" || req == "readwrite"
	allowReqWrite := req == "write" || req == "readwrite"
	allowRespRead := resp == "read" || resp == "readwrite"
	allowRespWrite := resp == "write" || resp == "readwrite"

	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().EncoderFilterCallbacks().Return(fakeEncoderCallbacks{}).Maybe()
	fc.EXPECT().GetProperty(mock.Anything).Return("", nil).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()
	// Error and reroute paths reach Envoy: failure details log, reloads clear
	// the route cache. Maybe keeps tests that stay silent unaffected.
	fc.EXPECT().Log(mock.Anything, mock.Anything).Return().Maybe()
	fc.EXPECT().ClearRouteCache().Return().Maybe()

	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache:          newInternalCache(),
		strictBodyAccess:       false,
		allowRequestBodyRead:   allowReqRead,
		allowRequestBodyWrite:  allowReqWrite,
		allowResponseBodyRead:  allowRespRead,
		allowResponseBodyWrite: allowRespWrite,
	}})
	require.NoError(t, err)

	return mustCastToContext(c)
}

func TestBridge_PassthroughNoop(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	status := f.DecodeHeaders(m, true)

	assert.Equal(t, api.Continue, status)
	select {
	case <-done:
	default:
		t.Fatal("handler did not run")
	}
}

func TestBridge_LocalReply(t *testing.T) {
	t.Parallel()

	var gotStatus int
	var gotBody string
	var gotHeaders http.Header

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		w.Header().Set("X-Reply", "yes")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("teapot"))
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)
	f.sendReply = func(ex *bridgeExchange, status int, body []byte) api.StatusType {
		gotStatus, gotBody, gotHeaders = status, string(body), ex.w.Header()
		return api.LocalReply
	}

	status := f.DecodeHeaders(m, true)

	assert.Equal(t, api.LocalReply, status)
	<-done
	assert.Equal(t, http.StatusTeapot, gotStatus)
	assert.Equal(t, "teapot", gotBody)
	assert.Equal(t, "yes", gotHeaders.Get("X-Reply"))
}

func TestBridge_BodyReadPauses(t *testing.T) {
	t.Parallel()

	read := make(chan string, 1)
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)

		b, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		read <- string(b)
	})

	m := fakeBridgeHeaders(t, "application/json")
	c := fakeBridgeContext(t, "read", "read")
	f := newBridgeFilter(c, h)
	f.errorHandler = func(_ bridgeContext, err error) api.StatusType {
		t.Logf("bridge error: %v", err)
		return api.Continue
	}

	status := f.DecodeHeaders(m, false)
	assert.Equal(t, api.StopAndBuffer, status)

	buf := mock_envoy.NewBufferInstance(t)
	buf.EXPECT().Bytes().Return([]byte(`{"a":1}`))

	status = f.DecodeData(buf, true)
	assert.Equal(t, api.Continue, status)
	<-done
	assert.Equal(t, `{"a":1}`, <-read)
}

func TestBridge_WriteThenProceedIsError(t *testing.T) {
	t.Parallel()

	var captured error
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		_, _ = w.Write([]byte("oops"))
		terminal{}.ServeHTTP(w, r)
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)
	f.errorHandler = func(_ bridgeContext, err error) api.StatusType {
		captured = err
		return api.Continue
	}

	_ = f.DecodeHeaders(m, true)
	<-done

	require.Error(t, captured)
	assert.ErrorIs(t, captured, ErrRuntime)
}

func TestBridge_ReplaceBody(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		r.Body = io.NopCloser(strings.NewReader("replaced"))
		terminal{}.ServeHTTP(w, r)
	})

	m := fakeBridgeHeaders(t, "application/json")
	c := fakeBridgeContext(t, "read", "read")
	f := newBridgeFilter(c, h)
	f.errorHandler = func(_ bridgeContext, err error) api.StatusType {
		t.Logf("bridge error: %v", err)
		return api.Continue
	}

	status := f.DecodeHeaders(m, false)
	assert.Equal(t, api.Continue, status)

	ex := f.exchange
	require.NotNil(t, ex)
	assert.Equal(t, "replaced", string(ex.replacedBody))

	ex.respReadyOnce.Do(func() { close(ex.respReady) })
	<-done
}

func TestBridge_OnDestroyUnblocksReader(t *testing.T) {
	t.Parallel()

	finished := make(chan error, 1)
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		finished <- err
	})

	m := fakeBridgeHeaders(t, "application/json")
	c := fakeBridgeContext(t, "read", "read")
	f := newBridgeFilter(c, h)
	f.exchange = f.newExchange(m, false)
	go f.exchange.runHandler()

	f.OnDestroy(api.Normal)

	select {
	case err := <-finished:
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrClientClosedRequest)
	case <-time.After(5 * time.Second):
		t.Fatal("handler goroutine did not finish after destroy")
	}
}

func TestBridge_PanicBecomesError(t *testing.T) {
	t.Parallel()

	var captured error
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		panic("boom")
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)
	f.errorHandler = func(_ bridgeContext, err error) api.StatusType {
		captured = err
		return api.Continue
	}

	f.DecodeHeaders(m, true)
	<-done

	require.Error(t, captured)
	assert.Contains(t, captured.Error(), "boom")
}

func TestBridge_EncodePassthroughAppliesHeaders(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		w.Header().Set("X-Hello", "world")
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	resp := mock_envoy.NewResponseHeaderMap(t)
	// Clean passthrough pins the encode trim: no Status, no Range, no
	// property Gets. Only the stashed X-Hello set reaches Envoy.
	resp.EXPECT().Get("X-Hello").Return("", false)
	resp.EXPECT().Del("X-Hello")
	resp.EXPECT().Add("X-Hello", "world")

	assert.Equal(t, api.Continue, f.EncodeHeaders(resp, true))
}

func TestBridge_EncodePassthroughSkipsWork(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))
	<-done

	// Bare mock: a clean passthrough must make zero Envoy calls.
	resp := mock_envoy.NewResponseHeaderMap(t)

	assert.Equal(t, api.Continue, f.EncodeHeaders(resp, true))
}

func TestBridge_ProceedThenHeaderMutation(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		terminal{}.ServeHTTP(w, r)
		w.Header().Set("X-After", "yes")
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))

	resp := mock_envoy.NewResponseHeaderMap(t)
	resp.EXPECT().Status().Return(200, true)
	resp.EXPECT().Range(mock.Anything).Return()
	resp.EXPECT().Get(HeaderXResponseBodyAccess).Return("", false).Maybe()
	resp.EXPECT().Get(HeaderContentType).Return("", false).Maybe()
	resp.EXPECT().Get(HeaderContentLength).Return("", false).Maybe()
	resp.EXPECT().Get(HeaderXContentOperation).Return("", false).Maybe()
	resp.EXPECT().Del("X-After")
	resp.EXPECT().Add("X-After", "yes")

	assert.Equal(t, api.Continue, f.EncodeHeaders(resp, true))
	<-done
}

func fakeRespHeaders(t *testing.T, status int) *mock_envoy.ResponseHeaderMap {
	t.Helper()

	m := mock_envoy.NewResponseHeaderMap(t)
	m.EXPECT().Status().Return(status, true)
	m.EXPECT().Range(mock.Anything).Return().Run(func(f func(string, string) bool) {
		f("Content-Type", "application/json")
	})
	m.EXPECT().Get(HeaderXResponseBodyAccess).Return("", false).Maybe()
	m.EXPECT().Get(HeaderContentType).Return("application/json", true).Maybe()
	m.EXPECT().Get(HeaderContentLength).Return("", false).Maybe()
	m.EXPECT().Get(HeaderXContentOperation).Return("", false).Maybe()
	m.EXPECT().Del(HeaderContentLength).Maybe()

	return m
}

func TestRecordingWriterSetTerminalUpstreamWins(t *testing.T) {
	t.Parallel()

	w := newRecordingWriter()
	w.Header().Set("x-conflict", "pre")
	w.Header().Set("x-only-pre", "1")

	upstream := http.Header{"X-Conflict": []string{"up"}}
	w.setTerminal(http.StatusOK, upstream, cloneHeader(w.Header()))

	assert.Equal(t, "up", w.Header().Get("x-conflict"))
	assert.Equal(t, "1", w.Header().Get("x-only-pre"))
	assert.Equal(t, http.StatusOK, w.Status())
}

func TestApplyHeaderSetsUpstreamWins(t *testing.T) {
	t.Parallel()

	live := mock_envoy.NewResponseHeaderMap(t)
	live.EXPECT().Get("X-Conflict").Return("up", true)
	live.EXPECT().Get("X-New").Return("", false)
	live.EXPECT().Del("X-New")
	live.EXPECT().Add("X-New", "1")

	current := http.Header{"X-Conflict": []string{"pre"}, "X-New": []string{"1"}}
	applyHeaderSets(live, current)
}

func TestBridge_ReadResponseBody(t *testing.T) {
	t.Parallel()

	read := make(chan string, 1)
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		terminal{}.ServeHTTP(w, r)

		rb, err := ResponseBody(r)
		if err != nil {
			return
		}
		b, err := io.ReadAll(rb)
		if err != nil {
			return
		}
		read <- string(b)
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "read")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))

	resp := fakeRespHeaders(t, 200)
	assert.Equal(t, api.StopAndBuffer, f.EncodeHeaders(resp, false))

	buf := mock_envoy.NewBufferInstance(t)
	buf.EXPECT().Bytes().Return([]byte(`{"up":true}`))

	assert.Equal(t, api.Continue, f.EncodeData(buf, true))
	<-done
	assert.Equal(t, `{"up":true}`, <-read)
}

func TestBridge_ReadOnlyResponseWriteIsError(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	called := make(chan error, 1)
	h := Use(func(next Handler) Handler {
		return Handler(func(c Context) error {
			defer close(done)
			if err := next(c); err != nil {
				return err
			}
			_, _ = c.Write([]byte(`{"new":true}`))
			return nil
		})
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "read")
	f := newBridgeFilter(c, h)
	f.errorHandler = func(_ bridgeContext, err error) api.StatusType {
		called <- err
		return api.LocalReply
	}

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))

	resp := fakeRespHeaders(t, 200)
	assert.Equal(t, api.LocalReply, f.EncodeHeaders(resp, false))
	<-done
	assert.ErrorIs(t, <-called, ErrOperationNotPermitted)
}

func TestBridge_ReplaceResponseBody(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		terminal{}.ServeHTTP(w, r)
		_, _ = w.Write([]byte(`{"new":true}`))
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "write")
	f := newBridgeFilter(c, h)
	f.sendReply = func(ex *bridgeExchange, status int, body []byte) api.StatusType {
		t.Error("replace must not become a local reply")
		return api.Continue
	}

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))

	resp := fakeRespHeaders(t, 200)
	assert.Equal(t, api.StopAndBuffer, f.EncodeHeaders(resp, false))
	<-done

	buf := mock_envoy.NewBufferInstance(t)
	buf.EXPECT().Set([]byte(`{"new":true}`)).Return(nil)

	assert.Equal(t, api.Continue, f.EncodeData(buf, true))
}

func TestBridgeAccessors_OutsideBridge(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)

	assert.NotPanics(t, func() { Log(r).Info("dropped") })
	assert.Nil(t, SharedCache(r))
	assert.NotPanics(t, func() { Reroute(r) })
}

func TestBridgeAccessors_Wired(t *testing.T) {
	t.Parallel()

	m := fakeBridgeHeaders(t, "")
	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().EncoderFilterCallbacks().Return(fakeEncoderCallbacks{}).Maybe()
	fc.EXPECT().GetProperty(mock.Anything).Return("", nil).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()
	// Lazy logger builds from cb on first Log: the wired Info below must
	// reach Envoy instead of hitting an unexpected mock call.
	fc.EXPECT().Log(mock.Anything, mock.Anything).Return().Maybe()

	c, err := newBridgeContext(fc, contextOptions{
		logger: logr.Discard(),
		config: &internalConfig{internalCache: newInternalCache()},
	})
	require.NoError(t, err)

	f := newBridgeFilter(mustCastToContext(c), HandlerFunc(func(w ResponseWriter, r *http.Request) {}))
	ex := f.newExchange(m, true)
	r := ex.req

	assert.NotNil(t, SharedCache(r))

	Reroute(r)
	assert.True(t, ex.reroute)

	assert.NotPanics(t, func() { Log(r).Info("wired") })
}

func TestBridge_PropagateRequestLine(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		r.URL.Path = "/staticreply"
		r.Host = "staticreply.svc"
		r.Method = http.MethodPost
	})

	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	live := mock_envoy.NewRequestHeaderMap(t)
	live.EXPECT().Method().Return(http.MethodGet).Maybe()
	live.EXPECT().Host().Return("example.com").Maybe()
	live.EXPECT().Path().Return("/").Maybe()
	live.EXPECT().Range(mock.Anything).Return().Maybe()
	live.EXPECT().SetPath("/staticreply")
	live.EXPECT().SetHost("staticreply.svc")
	live.EXPECT().SetMethod(http.MethodPost)
	live.EXPECT().Del(mock.Anything).Maybe()
	live.EXPECT().Add(mock.Anything, mock.Anything).Maybe()

	st := f.DecodeHeaders(live, true)
	<-done

	assert.Equal(t, api.Continue, st)
}

func TestBridge_DoubleProceedIsNoop(t *testing.T) {
	t.Parallel()

	calls := 0
	done := make(chan struct{})
	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		defer close(done)
		terminal{}.ServeHTTP(w, r)
		calls++
		terminal{}.ServeHTTP(w, r)
		calls++
	})

	m := fakeBridgeHeaders(t, "")
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.Continue, f.DecodeHeaders(m, true))

	ex := f.exchange
	require.NotNil(t, ex)
	assert.True(t, ex.nextCalled.Load())

	// Release the parked next() as EncodeHeaders would; OnDestroy closes
	// done so the trailing brDone send cannot block with no W listening.
	ex.respReadyOnce.Do(func() { close(ex.respReady) })
	f.OnDestroy(api.Normal)
	assert.Nil(t, f.exchange)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler goroutine did not finish after destroy")
	}
	assert.Equal(t, 2, calls)
}

func TestBridge_DecodeTrailersAppliesMutations(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		if string(body) != "chunk" {
			return
		}
		r.Header.Set("X-Trailer-Test", "applied")
	})

	m := fakeBridgeHeaders(t, "application/octet-stream")
	m.EXPECT().Del("X-Trailer-Test")
	m.EXPECT().Add("X-Trailer-Test", "applied")
	c := fakeBridgeContext(t, "read", "none")
	f := newBridgeFilter(c, h)

	assert.Equal(t, api.StopAndBuffer, f.DecodeHeaders(m, false))

	buf := mock_envoy.NewBufferInstance(t)
	buf.EXPECT().Bytes().Return([]byte("chunk"))

	assert.Equal(t, api.StopAndBuffer, f.DecodeData(buf, false))
	assert.Equal(t, api.Continue, f.DecodeTrailers(nil))
}

func TestBridgeExchange_PoolResetDropsState(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)

	ex := acquireBridgeExchange(h, r)
	oldDone, oldReady := ex.done, ex.respReady

	ex.w.WriteHeader(http.StatusCreated)
	_, _ = ex.w.Write([]byte("stale"))
	ex.w.Header().Set("X-Stale", "1")
	ex.attrs["x-stale"] = "1"
	ex.metricOps = append(ex.metricOps, MetricOp{Kind: MetricCounterAdd, Name: "stale"})
	ex.reqSnapshot = http.Header{"X-Stale": []string{"1"}}
	ex.respHeader = http.Header{"X-Stale": []string{"1"}}
	ex.preHeaders = http.Header{"X-Stale": []string{"1"}}
	ex.tls = TLSConnection{TLSVersion: "stale"}
	ex.tlsOK = true
	ex.nextCalled.Store(true)
	ex.proceedUpstream.Store(true)
	ex.earlyRespBody.Store(true)
	ex.failErr = errors.New("stale")
	ex.replacedBody = []byte("stale")
	ex.reroute = true
	ex.refreshRoute = true
	ex.hasOverride = true
	ex.drain = true
	ex.secretCache = map[string]secretResult{"s": {value: "v", found: true}}
	ex.feedBody([]byte("stale"), true)
	ex.feedRespBody([]byte("stale"), true)
	ex.signal(brNeedBody)
	ex.respCode = 599

	ex.gDone.Store(true)
	ex.destroy()
	releaseBridgeExchange(ex)

	next, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex2 := acquireBridgeExchange(h, next)

	assert.Equal(t, 0, ex2.w.Status())
	assert.Empty(t, ex2.w.body)
	assert.Empty(t, ex2.w.Header())
	assert.Empty(t, ex2.attrs)
	assert.Empty(t, ex2.metricOps)
	// Snapshot maps refill before every use instead of clearing on reset:
	// refilling must drop the stale keys (mirrors newExchange/EncodeHeaders).
	ex2.reqSnapshot = refillHeader(ex2.reqSnapshot, http.Header{"X-Fresh": {"1"}})
	assert.Equal(t, http.Header{"X-Fresh": {"1"}}, ex2.reqSnapshot)
	ex2.respHeader = refillHeader(ex2.respHeader, http.Header{"X-Fresh": {"1"}})
	assert.Equal(t, http.Header{"X-Fresh": {"1"}}, ex2.respHeader)
	ex2.preHeaders = refillHeader(ex2.preHeaders, http.Header{"X-Fresh": {"1"}})
	assert.Equal(t, http.Header{"X-Fresh": {"1"}}, ex2.preHeaders)
	assert.False(t, ex2.tlsOK)
	assert.Equal(t, TLSConnection{}, ex2.tls)
	assert.Nil(t, ex2.reqHeaderMap)
	assert.Nil(t, ex2.liveRespHeader)
	assert.Nil(t, ex2.streamInfo)
	assert.False(t, ex2.nextCalled.Load())
	assert.False(t, ex2.proceedUpstream.Load())
	assert.False(t, ex2.gDone.Load())
	assert.False(t, ex2.earlyRespBody.Load())
	assert.Nil(t, ex2.failErr)
	assert.Nil(t, ex2.replacedBody)
	assert.False(t, ex2.reroute)
	assert.False(t, ex2.refreshRoute)
	assert.False(t, ex2.hasOverride)
	assert.False(t, ex2.drain)
	assert.Empty(t, ex2.secretCache)
	assert.Empty(t, ex2.bodyChunks)
	assert.False(t, ex2.bodyEnded)
	assert.Empty(t, ex2.respChunks)
	assert.False(t, ex2.respEnded)
	assert.Equal(t, 0, ex2.respCode)
	assert.Equal(t, 0, len(ex2.sigCh))
	assert.Equal(t, 0, len(ex2.bodyWake))
	assert.Equal(t, 0, len(ex2.respWake))

	if ex2 == ex {
		assert.NotEqual(t, oldDone, ex2.done)
		assert.NotEqual(t, oldReady, ex2.respReady)
	}

	select {
	case <-ex2.done:
		t.Fatal("pooled done channel must be open")
	default:
	}

	select {
	case <-ex2.respReady:
		t.Fatal("pooled respReady channel must be open")
	default:
	}
}

func TestBridgeExchange_PoolAbortedDrops(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)

	ex := acquireBridgeExchange(h, r)
	// No gDone: handler still unwinding at destroy. Must not pool.
	ex.destroy()
	releaseBridgeExchange(ex)

	assert.False(t, ex.gDone.Load())
}

func TestBridgeExchange_ReuseThroughFilter(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	c := fakeBridgeContext(t, "none", "none")
	f := newBridgeFilter(c, h)

	m1 := fakeBridgeHeaders(t, "")
	ex1 := f.newExchange(m1, true)
	ex1.attrs["x-stale"] = "leak"
	ex1.w.Header().Set("X-Stale", "leak")
	ex1.gDone.Store(true)
	f.exchange = ex1
	f.OnDestroy(0)

	m2 := fakeBridgeHeaders(t, "")
	ex2 := f.newExchange(m2, true)

	_, leaked := ex2.attrs["x-stale"]
	assert.False(t, leaked, "attrs must not leak across pooled reuses")
	assert.Equal(t, "", ex2.w.Header().Get("X-Stale"))
	assert.True(t, ex2.hooksWired)
	assert.NotNil(t, ex2.gatedBody)
	assert.NotNil(t, ex2.respReader)
	assert.Equal(t, ex2, ex2.gatedBody.ex)

	host, ok := Attr(ex2.req, "request.host", "-")
	assert.True(t, ok)
	assert.Equal(t, "example.com", host)

	ex2.gDone.Store(true)
	f.exchange = ex2
	f.OnDestroy(0)
}

func TestHandlerWorkerPool_RunsTask(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	h := HandlerFunc(func(ResponseWriter, *http.Request) { close(done) })
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex := newBridgeExchange(h, r, newRecordingWriter(), bridgeHooks{})

	submitHandlerTask(ex)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pooled worker did not run the task")
	}

	assert.True(t, ex.gDone.Load())
}

func TestHandlerWorkerPool_PanicSurvival(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) { panic("boom") })
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex := newBridgeExchange(h, r, newRecordingWriter(), bridgeHooks{})

	submitHandlerTask(ex)

	select {
	case sig := <-ex.sigCh:
		assert.Equal(t, brFailed, sig)
		assert.ErrorContains(t, ex.failErr, "boom")
	case <-time.After(5 * time.Second):
		t.Fatal("panicked task produced no signal")
	}

	// The worker that absorbed the panic still serves: a second task runs.
	done := make(chan struct{})
	h2 := HandlerFunc(func(ResponseWriter, *http.Request) { close(done) })
	r2, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex2 := newBridgeExchange(h2, r2, newRecordingWriter(), bridgeHooks{})

	submitHandlerTask(ex2)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not survive the panic")
	}
}

func TestHandlerWorkerPool_OverflowRuns(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)

	block := HandlerFunc(func(w ResponseWriter, r *http.Request) {
		ex, _ := exchangeFrom(r)
		if ex == nil {
			return
		}

		select {
		case <-release:
		case <-ex.done:
		}
	})

	// Occupy every worker with a parked task.
	held := make([]*bridgeExchange, 0, numHandlerWorkers)
	for i := 0; i < numHandlerWorkers; i++ {
		r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		ex := newBridgeExchange(block, r, newRecordingWriter(), bridgeHooks{})
		ex.req = withBridgeExchange(r, ex)
		held = append(held, ex)
		submitHandlerTask(ex)
	}

	// Saturation overflows to a one-off goroutine: still runs, never stalls W.
	done := make(chan struct{})
	h := HandlerFunc(func(ResponseWriter, *http.Request) { close(done) })
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex := newBridgeExchange(h, r, newRecordingWriter(), bridgeHooks{})

	submitHandlerTask(ex)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("overflow task did not run under a saturated pool")
	}

	for _, ex := range held {
		ex.destroy()
	}
}

func TestRefillHeader_ReusesBacking(t *testing.T) {
	t.Parallel()

	dst := http.Header{"X-A": {"12345"}, "X-Stale": {"x"}}
	src := http.Header{"X-A": {"abcde"}, "X-B": {"y"}}

	out := refillHeader(dst, src)

	assert.Equal(t, []string{"abcde"}, out["X-A"])
	assert.Equal(t, []string{"y"}, out["X-B"])
	_, leaked := out["X-Stale"]
	assert.False(t, leaked, "keys absent from src must be dropped")

	// Value backing arrays survive across refills with equal-length values.
	second := http.Header{"X-A": {"vwxyz"}, "X-B": {"z"}}
	capBefore := cap(out["X-A"])
	out2 := refillHeader(out, second)
	assert.Equal(t, []string{"vwxyz"}, out2["X-A"])
	assert.GreaterOrEqual(t, cap(out2["X-A"]), capBefore)

	assert.Empty(t, refillHeader(nil, nil))
}

func TestBridge_AttrUndeclared(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()
	// Strict: no GetProperty expectation. Any prefetch call fails the test.
	// Log has a Maybe: only the migration warning may call it.
	fc.EXPECT().Log(mock.Anything, mock.Anything).Return().Maybe()
	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache: newInternalCache(),
		attrFetch:     &attrFetchConfig{},
	}})
	require.NoError(t, err)

	f := newBridgeFilter(mustCastToContext(c), h)
	ex := f.newExchange(fakeBridgeHeaders(t, ""), true)

	host, ok := Attr(ex.req, "request.host", "-")
	assert.True(t, ok)
	assert.Equal(t, "example.com", host)

	_, ok = Attr(ex.req, "xds.cluster_name", "-")
	assert.False(t, ok, "undeclared attrs stay absent")
}

func TestBridge_AttrDeclared(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()
	fc.EXPECT().GetProperty("xds.cluster_name").Return("egress", nil)
	fc.EXPECT().GetProperty("xds.route_name").Return("local", nil)
	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache: newInternalCache(),
		attrFetch:     &attrFetchConfig{fetch: []string{"xds.cluster_name", "xds.route_name"}},
	}})
	require.NoError(t, err)

	f := newBridgeFilter(mustCastToContext(c), h)
	ex := f.newExchange(fakeBridgeHeaders(t, ""), true)

	cluster, ok := Attr(ex.req, "xds.cluster_name", "-")
	assert.True(t, ok)
	assert.Equal(t, "egress", cluster)
}

func TestBridge_AttrUndeclaredWarnsOnce(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().StreamInfo().Return(nil).Maybe()
	logged := make(chan string, 8)
	fc.EXPECT().Log(mock.Anything, mock.Anything).Run(func(_ api.LogType, msg string) {
		logged <- msg
	}).Return().Maybe()
	c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
		internalCache: newInternalCache(),
		attrFetch:     &attrFetchConfig{},
	}})
	require.NoError(t, err)

	f := newBridgeFilter(mustCastToContext(c), h)
	ex := f.newExchange(fakeBridgeHeaders(t, ""), true)

	_, _ = Attr(ex.req, "xds.cluster_name", "-")
	_, _ = Attr(ex.req, "xds.route_name", "-")
	_, _ = Attr(ex.req, "request.host", "-")

	select {
	case msg := <-logged:
		assert.Contains(t, msg, "EnableEnvoyAttributes")
	case <-time.After(5 * time.Second):
		t.Fatal("undeclared attr miss produced no migration warning")
	}

	select {
	case extra := <-logged:
		t.Fatalf("warning must fire once, got extra: %q", extra)
	default:
	}
}

func TestBridge_TLSGated(t *testing.T) {
	t.Parallel()

	newTLSFilter := func(t *testing.T, enable bool) (*bridgeFilter, *mock_envoy.FilterCallbackHandler) {
		t.Helper()

		h := HandlerFunc(func(ResponseWriter, *http.Request) {})
		fc := mock_envoy.NewFilterCallbackHandler(t)
		fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
		fc.EXPECT().GetProperty(mock.Anything).Return("", nil).Maybe()
		fc.EXPECT().Log(mock.Anything, mock.Anything).Return().Maybe()
		si := &fakeStreamInfo{ssl: fakeSslConnection{}}
		fc.EXPECT().StreamInfo().Return(si).Maybe()
		c, err := newBridgeContext(fc, contextOptions{config: &internalConfig{
			internalCache:       newInternalCache(),
			enableDownstreamTLS: enable,
			attrFetch:           &attrFetchConfig{},
		}})
		require.NoError(t, err)

		return newBridgeFilter(mustCastToContext(c), h), fc
	}

	t.Run("declared snapshots", func(t *testing.T) {
		t.Parallel()

		f, _ := newTLSFilter(t, true)
		ex := f.newExchange(fakeBridgeHeaders(t, ""), true)

		conn, ok := TLS(ex.req)
		require.True(t, ok, "declared TLS must snapshot")
		assert.Equal(t, "ABCD", conn.Sha256PeerCertificateDigest)
	})

	t.Run("undeclared skips snapshot", func(t *testing.T) {
		t.Parallel()

		f, _ := newTLSFilter(t, false)
		ex := f.newExchange(fakeBridgeHeaders(t, ""), true)

		_, ok := TLS(ex.req)
		assert.False(t, ok, "undeclared TLS must stay absent")
	})
}

func TestResetBridgeExchange_DropsState(t *testing.T) {
	t.Parallel()

	h := HandlerFunc(func(ResponseWriter, *http.Request) {})
	r, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	ex := newBridgeExchange(h, r, newRecordingWriter(), bridgeHooks{})
	oldDone, oldReady := ex.done, ex.respReady

	ex.w.WriteHeader(http.StatusCreated)
	_, _ = ex.w.Write([]byte("stale"))
	ex.attrs["x-stale"] = "1"
	ex.metricOps = append(ex.metricOps, MetricOp{Kind: MetricCounterAdd, Name: "stale"})
	ex.reqSnapshot = http.Header{"X-Stale": {"1"}}
	ex.tlsOK = true
	ex.nextCalled.Store(true)
	ex.failErr = errors.New("stale")
	ex.replacedBody = []byte("stale")
	ex.reroute = true
	ex.drain = true
	ex.secretCache = map[string]secretResult{"s": {value: "v", found: true}}
	ex.feedBody([]byte("stale"), true)
	ex.signal(brNeedBody)

	resetBridgeExchange(ex)

	assert.Equal(t, 0, ex.w.Status())
	assert.Empty(t, ex.w.body)
	assert.Empty(t, ex.w.Header())
	assert.Empty(t, ex.attrs)
	assert.Empty(t, ex.metricOps)
	assert.False(t, ex.tlsOK)
	assert.False(t, ex.nextCalled.Load())
	assert.False(t, ex.gDone.Load())
	assert.Nil(t, ex.failErr)
	assert.Nil(t, ex.replacedBody)
	assert.False(t, ex.reroute)
	assert.False(t, ex.drain)
	assert.Empty(t, ex.secretCache)
	assert.Empty(t, ex.bodyChunks)
	assert.False(t, ex.bodyEnded)
	assert.Equal(t, 0, len(ex.sigCh))
	assert.NotEqual(t, oldDone, ex.done)
	assert.NotEqual(t, oldReady, ex.respReady)
	// Snapshot maps intentionally survive reset: every read site refills
	// them before use (refill drops stale keys), so clearing here would only
	// throw away reused backing arrays.
	assert.Equal(t, http.Header{"X-Stale": {"1"}}, ex.reqSnapshot)
}
