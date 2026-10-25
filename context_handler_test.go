package gonvoy

import (
	stdcontext "context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandlerContext builds a Context over a bridgeExchange outside
// Envoy. Hooks are intentionally nil-safe: every func is nil, mirroring how
// handler.go helpers behave outside the bridge. respReady is pre-closed so
// the internal proceed parks nowhere, simulating an arrived upstream response.
func newTestHandlerContext() *handlerContext {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	w := newRecordingWriter()
	ex := newBridgeExchange(nil, req, w, bridgeHooks{})
	req = withBridgeHooks(req, bridgeHooks{})
	req = withBridgeExchange(req, ex)
	ex.req = req
	ex.respReadyOnce.Do(func() { close(ex.respReady) })
	return newHandlerContext(w, req)
}

func TestHandlerContextResponseHeader(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()

	resp := c.Response()
	require.NotNil(t, resp)
	require.NotNil(t, resp.Header)

	resp.Header.Set("x-resp", "yes")
	assert.Equal(t, "yes", c.Response().Header.Get("x-resp"))
	assert.Equal(t, "yes", c.w.Header().Get("x-resp"))

	resp.Header.Add("x-multi", "1")
	resp.Header.Add("x-multi", "2")
	assert.Equal(t, []string{"1", "2"}, c.Response().Header.Values("x-multi"))

	resp.Header.Del("x-resp")
	assert.Empty(t, c.Response().Header.Get("x-resp"))

	require.NoError(t, c.NoContent(http.StatusCreated))
	assert.Equal(t, http.StatusCreated, c.Response().StatusCode)
	assert.Equal(t, "201 Created", c.Response().Status)
	assert.Nil(t, c.Response().Body)
}

func TestHandlerContextResponseFreshAfterTerminal(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()

	stale := c.Response()
	stale.Header.Set("x-stale", "1")

	upstream := http.Header{"X-Upstream": []string{"u"}}
	rw, ok := c.w.(*recordingWriter)
	require.True(t, ok, "handlerContext writer is a recording writer")
	rw.setTerminal(http.StatusOK, upstream, cloneHeader(c.w.Header()))

	fresh := c.Response()
	assert.Equal(t, "u", fresh.Header.Get("X-Upstream"))
	assert.Equal(t, "1", fresh.Header.Get("x-stale"))
	assert.Equal(t, http.StatusOK, fresh.StatusCode)
	assert.Equal(t, "200 OK", fresh.Status)
}

func TestHandlerContextNilRequestHeader(t *testing.T) {
	t.Parallel()

	r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	require.NoError(t, err)
	r.Header = nil
	c := newHandlerContext(newRecordingWriter(), r)

	require.NotPanics(t, func() { c.Request().Header.Set("x-a", "1") })
	assert.Equal(t, "1", c.Request().Header.Get("x-a"))
}

func TestHandlerContextHeaderFlow(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()

	c.Request().Header.Set("x-a", "1")
	assert.Equal(t, "1", c.Request().Header.Get("x-a"))

	assert.Equal(t, http.MethodGet, c.Method())
	assert.Equal(t, "/", c.Path())
	assert.Equal(t, "example.com", c.Host())

	c.SetMethod(http.MethodPost)
	c.SetPath("/staticreply")
	c.SetHost("staticreply.svc")
	assert.Equal(t, http.MethodPost, c.Method())
	assert.Equal(t, "/staticreply", c.Path())
	assert.Equal(t, "staticreply.svc", c.Host())

	c.Response().Header.Set("x-resp", "yes")
	assert.Equal(t, "yes", c.w.Header().Get("x-resp"))

	require.NoError(t, c.forwardUpstream())
	assert.True(t, c.ex.nextCalled.Load())

	// Proceeding is single-shot: calling twice is a no-op.
	require.NoError(t, c.forwardUpstream())
}

func TestHandlerContextWriteThenProceed(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()

	// A response written before proceeding competes with going upstream.
	// The bridge answers 500 instead of picking a winner silently.
	require.NoError(t, c.String(http.StatusOK, "hi"))

	err := c.forwardUpstream()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRuntime)
}

func TestHandlerContextNoProceed(t *testing.T) {
	t.Parallel()

	t.Run("direct view does not proceed", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()

		assert.False(t, c.ex.nextCalled.Load())
		assert.False(t, c.ex.proceedUpstream.Load())
		assert.Equal(t, 0, c.Status())
	})

	t.Run("handler without next completes", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()

		h := Handler(func(ec Context) error {
			ec.Request().Header.Set("x-b", "2")
			return nil
		})

		require.NotPanics(t, func() { h.ServeHTTP(c.w, c.r) })

		assert.Equal(t, "2", c.Request().Header.Get("x-b"))
		assert.False(t, c.ex.nextCalled.Load())
		assert.Nil(t, c.ex.failErr)
	})

	t.Run("handler error surfaces without next", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()

		h := Handler(func(Context) error { return ErrUnauthorized })

		require.NotPanics(t, func() { h.ServeHTTP(c.w, c.r) })

		assert.ErrorIs(t, c.ex.failErr, ErrUnauthorized)
	})

	t.Run("handler outside bridge is nil-safe", func(t *testing.T) {
		t.Parallel()

		r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		require.NoError(t, err)

		h := Use(func(next Handler) Handler {
			return Handler(func(ec Context) error {
				ec.Request().Header.Set("x-c", "3")
				assert.NoError(t, next(ec))
				return nil
			})
		})

		require.NotPanics(t, func() { h.ServeHTTP(newRecordingWriter(), r) })
		assert.Equal(t, "3", r.Header.Get("x-c"))
	})
}

// newTestHandlerContextWithBody builds a Context whose request body is a
// gated reader fed with body and closed, readable like a buffered request.
func newTestHandlerContextWithBody(t *testing.T, body, contentType string) *handlerContext {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "http://example.com/", nil)
	require.NoError(t, err)
	req.Header.Set(HeaderContentType, contentType)

	w := newRecordingWriter()
	ex := newBridgeExchange(nil, req, w, bridgeHooks{})
	req = withBridgeHooks(req, bridgeHooks{})
	req = withBridgeExchange(req, ex)

	gated := &gatedBodyReader{ex: ex, enabled: true}
	ex.gatedBody = gated
	req.Body = gated
	ex.req = req
	ex.feedBody([]byte(body), true)
	ex.respReadyOnce.Do(func() { close(ex.respReady) })

	return newHandlerContext(w, req)
}

// newTestHandlerContextNoBodyAccess builds a Context whose request body
// reads are gated off and whose upstream response reader is absent, so both
// Body and ResponseBody report ErrOperationNotPermitted.
func newTestHandlerContextNoBodyAccess() *handlerContext {
	c := newTestHandlerContext()

	gated := &gatedBodyReader{ex: c.ex, enabled: false}
	c.ex.gatedBody = gated
	c.r.Body = gated
	c.ex.req = c.r

	return c
}

// recordedBody returns the bytes the context wrote into its recording writer.
func recordedBody(t *testing.T, c *handlerContext) string {
	t.Helper()

	rw, ok := c.w.(*recordingWriter)
	require.True(t, ok, "handlerContext writer is a recording writer")

	return string(rw.body)
}

func TestHandlerContextReply(t *testing.T) {
	t.Parallel()

	t.Run("json sets status content type and body", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()
		require.NoError(t, c.JSON(http.StatusOK, map[string]string{"ok": "true"}))

		assert.Equal(t, http.StatusOK, c.Status())
		assert.Equal(t, MIMEApplicationJSONCharsetUTF8, c.w.Header().Get(HeaderContentType))
		assert.JSONEq(t, `{"ok":"true"}`, recordedBody(t, c))
	})

	t.Run("string sets status content type and body", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()
		require.NoError(t, c.String(http.StatusCreated, "hi"))

		assert.Equal(t, http.StatusCreated, c.Status())
		assert.Equal(t, MIMETextPlainCharsetUTF8, c.w.Header().Get(HeaderContentType))
		assert.Equal(t, "hi", recordedBody(t, c))
	})

	t.Run("no content writes header only", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()
		require.NoError(t, c.NoContent(http.StatusNoContent))

		assert.Equal(t, http.StatusNoContent, c.Status())
		assert.Empty(t, recordedBody(t, c))
	})
}

func TestHandlerContextBind(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContextWithBody(t, `{"a":1}`, MIMEApplicationJSON)

	var v map[string]any
	require.NoError(t, c.Bind(&v))
	assert.Equal(t, float64(1), v["a"])
}

func TestHandlerContextBodyGating(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContextNoBodyAccess()
	_, err := c.Body().Read(make([]byte, 1))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
}

func TestHandlerContextResponseGating(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContextNoBodyAccess()
	_, err := c.ResponseBody()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
}

func TestHandlerContextPlatform(t *testing.T) {
	t.Parallel()

	t.Run("delegates stay nil-safe on the bridge", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()

		c.Log().Info("platform", "where", "bridge")
		c.Stats().Counter("req").Increment(1)
		assert.Nil(t, c.Cache())

		if _, ok := c.Attr("request.host", "-"); !ok {
			t.Log("outside bridge, default returned")
		}

		require.NotPanics(t, func() {
			c.Reroute()
			c.RefreshRoute()
			c.OverrideUpstream("10.0.0.9:8080", false)
			c.Drain()
		})
		assert.True(t, c.ex.refreshRoute)

		_, ok := c.Secret("missing")
		assert.False(t, ok)

		_, ok = c.TLS()
		assert.False(t, ok)

		c.r.URL.RawQuery = "a=1"
		assert.Equal(t, "1", c.QueryParam("a"))
		assert.Empty(t, c.QueryParam("missing"))

		_, err := ConfigOf[string](c)
		require.Error(t, err)
	})

	t.Run("typed config round-trips through the context", func(t *testing.T) {
		t.Parallel()

		c := newTestHandlerContext()
		c.r = c.r.WithContext(stdcontext.WithValue(c.r.Context(), bridgeConfigKey{}, "cfg"))

		got, err := ConfigOf[string](c)
		require.NoError(t, err)
		assert.Equal(t, "cfg", got)

		_, err = ConfigOf[int](c)
		require.Error(t, err)
	})

	t.Run("delegates stay nil-safe outside the bridge", func(t *testing.T) {
		t.Parallel()

		r, err := http.NewRequest(http.MethodGet, "http://example.com/?a=1", nil)
		require.NoError(t, err)
		c := newHandlerContext(newRecordingWriter(), r)

		require.NotPanics(t, func() {
			_ = c.Log()
			_ = c.Stats()
			_ = c.Cache()
			_, _ = c.Attr("request.host", "-")
			c.Reroute()
			c.RefreshRoute()
			c.OverrideUpstream("10.0.0.9:8080", false)
			c.Drain()
			_, _ = c.Secret("missing")
			_, _ = c.TLS()
		})
		assert.Equal(t, "1", c.QueryParam("a"))

		_, err = ConfigOf[string](c)
		require.Error(t, err)
	})
}

func TestHandlerContextWrite(t *testing.T) {
	t.Parallel()

	w := newRecordingWriter()
	r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	require.NoError(t, err)
	cw := newHandlerContext(w, r)

	n, err := cw.Write([]byte(`{"a":1}`))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	assert.Equal(t, `{"a":1}`, string(w.body))
	assert.False(t, w.explicit)
	assert.Equal(t, http.StatusOK, cw.Status())
}

func TestHandlerContextBindNonJSON(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContextWithBody(t, `a=1`, MIMEApplicationForm)

	var v map[string]any
	err := c.Bind(&v)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
	assert.True(t, strings.Contains(c.r.Header.Get(HeaderContentType), MIMEApplicationForm))
}
