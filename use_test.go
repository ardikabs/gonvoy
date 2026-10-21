package gonvoy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUse_Order(t *testing.T) {
	t.Parallel()

	var order []string
	mk := func(name string) func(Handler) Handler {
		return func(next Handler) Handler {
			return HandlerFunc(func(w ResponseWriter, r *http.Request) {
				order = append(order, name+":before")
				next.ServeHTTP(w, r)
				order = append(order, name+":after")
			})
		}
	}

	h := Use(mk("outer"), mk("inner"))
	h.ServeHTTP(newRecordingWriter(), &http.Request{})

	assert.Equal(t, []string{"outer:before", "inner:before", "inner:after", "outer:after"}, order)
}

func TestUse_NilPanics(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { Use(nil) })
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
