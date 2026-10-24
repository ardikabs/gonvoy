package gonvoy

import (
	"net/http"
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
	_, err := c.w.Write([]byte("hi"))
	require.NoError(t, err)

	err = c.Next()
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
