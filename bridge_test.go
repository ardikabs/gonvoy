package gonvoy

import (
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
	resp.EXPECT().Status().Return(200, true)
	resp.EXPECT().Range(mock.Anything).Return()
	resp.EXPECT().Del("X-Hello")
	resp.EXPECT().Add("X-Hello", "world")

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
	m.EXPECT().Get(HeaderContentLength).Return("", false).Maybe()
	m.EXPECT().Del(HeaderContentLength).Maybe()

	return m
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
