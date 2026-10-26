package gonvoy

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUse_Order(t *testing.T) {
	t.Parallel()

	var order []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return Handler(func(c Context) error {
				order = append(order, name+":before")
				err := next(c)
				order = append(order, name+":after")
				return err
			})
		}
	}

	h := Use(mk("outer"), mk("inner"))
	require.NoError(t, h(newTestHandlerContext()))
	assert.Equal(t, []string{"outer:before", "inner:before", "inner:after", "outer:after"}, order)
}

func TestUse_NilPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { Use(nil) })
}

// TestUse_ExplicitFinal pins the onion order for Context-based handlers:
// a final Handler nested inside middleware directly, and the same order
// through Chain.
func TestUse_ExplicitFinal(t *testing.T) {
	t.Parallel()

	var order []string
	outer := Middleware(func(next Handler) Handler {
		return Handler(func(c Context) error {
			order = append(order, "outer:before")
			err := next(c)
			order = append(order, "outer:after")
			return err
		})
	})
	handler := Handler(func(c Context) error {
		order = append(order, "handler")
		return nil
	})

	h := outer(handler)
	require.NoError(t, h(newTestHandlerContext()))
	assert.Equal(t, []string{"outer:before", "handler", "outer:after"}, order)

	order = nil
	h2 := Chain(handler, outer)
	require.NoError(t, h2(newTestHandlerContext()))
	assert.Equal(t, []string{"outer:before", "handler", "outer:after"}, order)
}

func TestOption_Apply(t *testing.T) {
	t.Parallel()

	t.Run("body rules", func(t *testing.T) {
		t.Parallel()

		o := Options{}
		ReadRequestBody()(&o)
		assert.Equal(t, BodyRuleRead, o.Body.Request)

		o = Options{}
		WriteRequestBody()(&o)
		assert.Equal(t, BodyRuleReadWrite, o.Body.Request)

		o = Options{}
		ReadResponseBody()(&o)
		assert.Equal(t, BodyRuleRead, o.Body.Response)

		o = Options{}
		WriteResponseBody()(&o)
		assert.Equal(t, BodyRuleReadWrite, o.Body.Response)
	})

	t.Run("access gate", func(t *testing.T) {
		t.Parallel()

		o := Options{Body: BodyPolicy{Access: BodyAccessOpen}}
		RequireBodyPermissionHeader()(&o)
		assert.Equal(t, BodyAccessHeaderGated, o.Body.Access)

		o = Options{}
		OpenBodyAccess()(&o)
		assert.Equal(t, BodyAccessOpen, o.Body.Access)
	})

	t.Run("config error action", func(t *testing.T) {
		t.Parallel()

		o := Options{}
		OnConfigError(UseListener)(&o)
		assert.Equal(t, MergeOnErrorFallbackToParent, o.Merge.OnError)

		o = Options{}
		OnConfigError(Stop)(&o)
		assert.Equal(t, MergeOnErrorPanic, o.Merge.OnError)
	})

	t.Run("route merge prefix keep", func(t *testing.T) {
		t.Parallel()

		o := Options{}
		RouteConfigWins()(&o)
		assert.True(t, o.Merge.ChildWins)

		o = Options{}
		ReloadRoute()(&o)
		assert.True(t, o.Route.ReloadOnHeaderChange)

		o = Options{}
		MetricPrefix("p_")(&o)
		assert.Equal(t, "p_", o.MetricsPrefix)

		o = Options{}
		WithConfig(42)(&o)
		assert.Equal(t, 42, o.Config)

		o = Options{}
		KeepContentLength(KeepLengthBoth)(&o)
		assert.Equal(t, KeepLengthBoth, o.Body.KeepContentLength)
	})
}

// TestUse_MiddlewareReachesUpstream pins the explicit chaining flow: a
// Middleware calls next to proceed, the innermost next is the internal
// terminal, and code after next runs when the upstream response arrives.
func TestUse_MiddlewareReachesUpstream(t *testing.T) {
	t.Parallel()

	var order []string
	auth := Middleware(func(next Handler) Handler {
		return Handler(func(c Context) error {
			order = append(order, "auth:before")
			if err := next(c); err != nil {
				return err
			}
			order = append(order, "auth:after")
			return nil
		})
	})
	log := Middleware(func(next Handler) Handler {
		return Handler(func(c Context) error {
			order = append(order, "log:before")
			if err := next(c); err != nil {
				return err
			}
			order = append(order, "log:after")
			return nil
		})
	})

	c := newTestHandlerContext()
	h := Use(auth, log)
	require.NoError(t, h(c))

	assert.True(t, c.ex.nextCalled.Load(), "innermost next must reach the internal terminal")
	assert.Equal(t, []string{"auth:before", "log:before", "log:after", "auth:after"}, order)
}

// TestUse_SingleMiddlewareObservesResponse pins that one Middleware in Use
// is enough to observe the upstream response: next parks, then after-code
// sees the status through the same Context.
func TestUse_SingleMiddlewareObservesResponse(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()
	c.ex.respCode = http.StatusTeapot
	c.ex.respHeader = map[string][]string{"X-Up": {"1"}}

	var sawStatus int
	h := Use(func(next Handler) Handler {
		return Handler(func(c Context) error {
			if err := next(c); err != nil {
				return err
			}
			sawStatus = c.Status()
			return nil
		})
	})

	require.NoError(t, h(c))
	assert.True(t, c.ex.nextCalled.Load())
	assert.Equal(t, http.StatusTeapot, sawStatus)
	assert.Equal(t, http.StatusTeapot, c.Status())
}

// TestChain_ExplicitFinal pins Chain: layers wrap an explicit final Handler.
// An unanswered final passes through without parking for a response nobody
// asked for; pre-next response header sets still reach the upstream reply
// through the encode fast path. An answered final short-circuits without
// forwarding.
func TestChain_ExplicitFinal(t *testing.T) {
	t.Parallel()

	var order []string
	outer := Middleware(func(next Handler) Handler {
		return Handler(func(c Context) error {
			order = append(order, "outer:before")
			err := next(c)
			order = append(order, "outer:after")
			return err
		})
	})
	final := Handler(func(c Context) error {
		order = append(order, "final")
		return nil
	})

	c := newTestHandlerContext()
	require.NoError(t, Chain(final, outer)(c))

	assert.Equal(t, []string{"outer:before", "final", "outer:after"}, order)
	assert.False(t, c.ex.nextCalled.Load(), "unanswered final passes through without parking")

	order = nil
	c2 := newTestHandlerContext()
	answered := Handler(func(c Context) error {
		order = append(order, "final")
		return c.String(200, "stub")
	})
	require.NoError(t, Chain(answered, outer)(c2))

	assert.Equal(t, []string{"outer:before", "final", "outer:after"}, order)
	assert.False(t, c2.ex.nextCalled.Load(), "answered final must not reach upstream")
}

// TestChain_NilPanics pins contract violations for Chain.
func TestChain_NilPanics(t *testing.T) {
	t.Parallel()

	final := Handler(func(Context) error { return nil })

	require.Panics(t, func() { Chain(nil) })
	require.Panics(t, func() { Chain(final, nil) })
}

// TestChain_UnansweredFinalForwardsUpstream pins the passthrough guarantee:
// a final that leaves the request unanswered still forwards upstream,
// without parking for a response nobody observes.
func TestChain_UnansweredFinalForwardsUpstream(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()
	final := Handler(func(Context) error { return nil })

	require.NoError(t, Chain(final)(c))
	assert.False(t, c.ex.nextCalled.Load(), "unanswered final passes through without parking")
}

// TestChain_ReplacedBodyStillReachesTerminal pins the carve-out: a final
// that swaps r.Body without proceeding still goes through the terminal hop,
// which alone streams the replacement upstream.
func TestChain_ReplacedBodyStillReachesTerminal(t *testing.T) {
	t.Parallel()

	c := newTestHandlerContext()
	final := Handler(func(c Context) error {
		c.Request().Body = io.NopCloser(strings.NewReader("replacement"))
		return nil
	})

	require.NoError(t, Chain(final)(c))
	assert.True(t, c.ex.nextCalled.Load(), "swapped body must stream through the terminal hop")
}

// TestChain_SkippedMiddlewareStillForwards pins the outer-boundary passthrough:
// an outer layer that returns without calling next still forwards the request
// upstream, without parking for a response nobody observes.
func TestChain_SkippedMiddlewareStillForwards(t *testing.T) {
	t.Parallel()

	var order []string
	outer := Middleware(func(Handler) Handler {
		return Handler(func(c Context) error {
			order = append(order, "outer")
			return nil
		})
	})
	final := Handler(func(Context) error {
		order = append(order, "final")
		return nil
	})

	c := newTestHandlerContext()
	require.NoError(t, Chain(final, outer)(c))

	assert.Equal(t, []string{"outer"}, order)
	assert.False(t, c.ex.nextCalled.Load(), "skipped chain passes through without parking")
}
