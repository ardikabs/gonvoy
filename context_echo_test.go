package gonvoy

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestEchoContext builds an EchoContext over a bridgeExchange outside
// Envoy. Hooks are intentionally nil-safe: every func is nil, mirroring how
// handler.go helpers behave outside the bridge. respReady is pre-closed so
// Next parks nowhere, simulating an arrived upstream response.
func newTestEchoContext() *echoContext {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	w := newRecordingWriter()
	ex := newBridgeExchange(nil, req, w, bridgeHooks{})
	req = withBridgeHooks(req, bridgeHooks{})
	req = withBridgeExchange(req, ex)
	ex.req = req
	ex.respReadyOnce.Do(func() { close(ex.respReady) })
	return newEchoContext(w, req)
}

func TestEchoContextHeaderFlow(t *testing.T) {
	t.Parallel()

	c := newTestEchoContext()

	c.SetRequestHeader("x-a", "1")
	assert.Equal(t, "1", c.Header("x-a"))

	assert.Equal(t, http.MethodGet, c.Method())
	assert.Equal(t, "/", c.Path())
	assert.Equal(t, "example.com", c.Host())

	c.SetMethod(http.MethodPost)
	c.SetPath("/staticreply")
	c.SetHost("staticreply.svc")
	assert.Equal(t, http.MethodPost, c.Method())
	assert.Equal(t, "/staticreply", c.Path())
	assert.Equal(t, "staticreply.svc", c.Host())

	c.SetHeader("x-resp", "yes")
	assert.Equal(t, "yes", c.w.Header().Get("x-resp"))

	require.NoError(t, c.Next())
	assert.True(t, c.ex.nextCalled.Load())

	// Next is single-shot: calling twice is a no-op.
	require.NoError(t, c.Next())
}

func TestEchoContextWriteThenNext(t *testing.T) {
	t.Parallel()

	c := newTestEchoContext()

	// A response written before Next competes with proceeding upstream.
	// The bridge answers 500 instead of picking a winner silently.
	require.NoError(t, c.String(http.StatusOK, "hi"))

	err := c.Next()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRuntime)
}

func TestEchoContextNoNext(t *testing.T) {
	t.Parallel()

	t.Run("direct view does not proceed", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()

		assert.False(t, c.ex.nextCalled.Load())
		assert.False(t, c.ex.proceedUpstream.Load())
		assert.Equal(t, 0, c.Status())
	})

	t.Run("echo handler without next completes", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()

		h := Echo(func(ec EchoContext) error {
			ec.SetRequestHeader("x-b", "2")
			return nil
		})

		require.NotPanics(t, func() { h.ServeHTTP(c.w, c.r) })

		assert.Equal(t, "2", c.Header("x-b"))
		assert.False(t, c.ex.nextCalled.Load())
		assert.Nil(t, c.ex.failErr)
	})

	t.Run("echo handler error surfaces without next", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()

		h := Echo(func(EchoContext) error { return ErrUnauthorized })

		require.NotPanics(t, func() { h.ServeHTTP(c.w, c.r) })

		assert.ErrorIs(t, c.ex.failErr, ErrUnauthorized)
	})

	t.Run("echo handler outside bridge is nil-safe", func(t *testing.T) {
		t.Parallel()

		r, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
		require.NoError(t, err)

		h := Echo(func(ec EchoContext) error {
			ec.SetRequestHeader("x-c", "3")
			assert.NoError(t, ec.Next())
			return nil
		})

		require.NotPanics(t, func() { h.ServeHTTP(newRecordingWriter(), r) })
		assert.Equal(t, "3", r.Header.Get("x-c"))
	})
}

// newTestEchoContextWithBody builds an EchoContext whose request body is a
// gated reader fed with body and closed, readable like a buffered request.
func newTestEchoContextWithBody(t *testing.T, body, contentType string) *echoContext {
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

	return newEchoContext(w, req)
}

// newTestEchoContextNoBodyAccess builds an EchoContext whose request body
// reads are gated off and whose upstream response reader is absent, so both
// Body and ResponseBody report ErrOperationNotPermitted.
func newTestEchoContextNoBodyAccess() *echoContext {
	c := newTestEchoContext()

	gated := &gatedBodyReader{ex: c.ex, enabled: false}
	c.ex.gatedBody = gated
	c.r.Body = gated
	c.ex.req = c.r

	return c
}

// recordedBody returns the bytes the context wrote into its recording writer.
func recordedBody(t *testing.T, c *echoContext) string {
	t.Helper()

	rw, ok := c.w.(*recordingWriter)
	require.True(t, ok, "echoContext writer is a recording writer")

	return string(rw.body)
}

func TestEchoContextReply(t *testing.T) {
	t.Parallel()

	t.Run("json sets status content type and body", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()
		require.NoError(t, c.JSON(http.StatusOK, map[string]string{"ok": "true"}))

		assert.Equal(t, http.StatusOK, c.Status())
		assert.Equal(t, MIMEApplicationJSONCharsetUTF8, c.w.Header().Get(HeaderContentType))
		assert.JSONEq(t, `{"ok":"true"}`, recordedBody(t, c))
	})

	t.Run("string sets status content type and body", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()
		require.NoError(t, c.String(http.StatusCreated, "hi"))

		assert.Equal(t, http.StatusCreated, c.Status())
		assert.Equal(t, MIMETextPlainCharsetUTF8, c.w.Header().Get(HeaderContentType))
		assert.Equal(t, "hi", recordedBody(t, c))
	})

	t.Run("no content writes header only", func(t *testing.T) {
		t.Parallel()

		c := newTestEchoContext()
		require.NoError(t, c.NoContent(http.StatusNoContent))

		assert.Equal(t, http.StatusNoContent, c.Status())
		assert.Empty(t, recordedBody(t, c))
	})
}

func TestEchoContextBind(t *testing.T) {
	t.Parallel()

	c := newTestEchoContextWithBody(t, `{"a":1}`, MIMEApplicationJSON)

	var v map[string]any
	require.NoError(t, c.Bind(&v))
	assert.Equal(t, float64(1), v["a"])
}

func TestEchoContextBodyGating(t *testing.T) {
	t.Parallel()

	c := newTestEchoContextNoBodyAccess()
	_, err := c.Body().Read(make([]byte, 1))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
}

func TestEchoContextResponseGating(t *testing.T) {
	t.Parallel()

	c := newTestEchoContextNoBodyAccess()
	_, err := c.ResponseBody()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
}

func TestEchoContextBindNonJSON(t *testing.T) {
	t.Parallel()

	c := newTestEchoContextWithBody(t, `a=1`, MIMEApplicationForm)

	var v map[string]any
	err := c.Bind(&v)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOperationNotPermitted)
	assert.True(t, strings.Contains(c.r.Header.Get(HeaderContentType), MIMEApplicationForm))
}
