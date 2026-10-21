package gonvoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDecoderCallbacks struct {
	api.DecoderFilterCallbacks
}

type fakeEncoderCallbacks struct {
	api.EncoderFilterCallbacks
}

type misuseFilter struct{}

func (misuseFilter) OnRequestHeader(c Context) error {
	_ = c.ResponseHeader()
	return nil
}

type panicFilter struct{}

func (panicFilter) OnRequestHeader(c Context) error {
	panic("phase on panic")
}

func TestFilterChain_PanicReachesFinalize(t *testing.T) {
	t.Parallel()

	c := fakeChainContext(t)

	var captured error
	mgr := newHttpFilterManager(c)
	mgr.SetErrorHandler(func(_ Context, err error) api.StatusType {
		captured = err
		return api.Continue
	})
	mgr.AddFilter(panicFilter{})

	var res *HttpFilterResult
	require.NotPanics(t, func() {
		res = mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
	})
	require.Error(t, res.Err)
	assert.ErrorIs(t, res.Err, ErrRuntime)
	assert.ErrorIs(t, captured, ErrRuntime)
}

type recordFilter struct {
	calls *[]string
	name  string
}

func (f *recordFilter) OnRequestHeader(c Context) error {
	*f.calls = append(*f.calls, f.name+":request")
	return nil
}

func (f *recordFilter) OnResponseHeader(c Context) error {
	*f.calls = append(*f.calls, f.name+":response")
	return nil
}

type fakeComposer struct {
	filters []Filter
	err     error
	self    *[]string
}

func (f *fakeComposer) Use(c Context) ([]Filter, error) {
	if f.self != nil {
		*f.self = append(*f.self, "use")
	}
	return f.filters, f.err
}

func (f *fakeComposer) OnRequestHeader(c Context) error {
	if f.self != nil {
		*f.self = append(*f.self, "own-phase")
	}
	return nil
}

func fakeChainContext(t *testing.T) *context {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	fc.EXPECT().DecoderFilterCallbacks().Return(fakeDecoderCallbacks{}).Maybe()
	fc.EXPECT().EncoderFilterCallbacks().Return(fakeEncoderCallbacks{}).Maybe()

	c, err := NewContext(fc, contextOptions{config: &internalConfig{internalCache: newInternalCache()}})
	require.NoError(t, err)
	return mustCastToContext(c)
}

func TestFilterChain_MisuseBecomesError(t *testing.T) {
	t.Parallel()

	c := fakeChainContext(t)

	var captured error
	mgr := newHttpFilterManager(c)
	mgr.SetErrorHandler(func(_ Context, err error) api.StatusType {
		captured = err
		return api.Continue
	})
	mgr.AddFilter(misuseFilter{})

	var res *HttpFilterResult
	require.NotPanics(t, func() {
		res = mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
	})
	require.Error(t, res.Err)
	assert.ErrorIs(t, res.Err, ErrOperationNotPermitted)
	assert.ErrorIs(t, captured, ErrOperationNotPermitted)
	assert.Equal(t, api.Continue, res.Status)
}

func TestFilterChain_Order(t *testing.T) {
	t.Parallel()

	calls := []string{}
	mgr := newHttpFilterManager(fakeChainContext(t))
	mgr.AddFilter(&recordFilter{calls: &calls, name: "first"})
	mgr.AddFilter(&recordFilter{calls: &calls, name: "second"})

	decode := mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
	assert.NoError(t, decode.Err)

	encode := mgr.ServeEncodeFilter(fakeEncodeHeadersPhase())
	assert.NoError(t, encode.Err)

	assert.Equal(t,
		[]string{"first:request", "second:request", "second:response", "first:response"},
		calls)
}

func TestComposer_BuildsChain(t *testing.T) {
	t.Run("use filters run in chain order", func(t *testing.T) {
		t.Parallel()

		calls := []string{}
		c := fakeChainContext(t)
		comp := &fakeComposer{filters: []Filter{
			&recordFilter{calls: &calls, name: "first"},
			&recordFilter{calls: &calls, name: "second"},
		}}

		mgr, err := buildFilterChainManager(c, func() Filter { return comp })
		require.NoError(t, err)

		decode := mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
		assert.NoError(t, decode.Err)
		assert.Equal(t, []string{"first:request", "second:request"}, calls)
	})

	t.Run("composer runs first in its own chain", func(t *testing.T) {
		t.Parallel()

		self := []string{}
		c := fakeChainContext(t)
		comp := &fakeComposer{self: &self}

		mgr, err := buildFilterChainManager(c, func() Filter { return comp })
		require.NoError(t, err)

		decode := mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
		assert.NoError(t, decode.Err)
		assert.Equal(t, []string{"use", "own-phase"}, self)
	})

	t.Run("use error ignores filter", func(t *testing.T) {
		t.Parallel()

		c := fakeChainContext(t)
		comp := &fakeComposer{err: assert.AnError}

		mgr, err := buildFilterChainManager(c, func() Filter { return comp })
		require.NoError(t, err)

		decode := mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
		assert.NoError(t, decode.Err)
		assert.Equal(t, ActionSkip, decode.Action)
	})
}

func TestNewFilterChainFactory(t *testing.T) {
	t.Run("nil constructor panics", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			NewFilterChainFactory(nil)
		})
		assert.Panics(t, func() {
			NewFilterChainFactory(func() Filter { return nil })
		})
	})

	t.Run("valid constructor returns factory", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, NewFilterChainFactory(func() Filter { return misuseFilter{} }))
	})
}

type completeFilter struct {
	done *bool
}

func (f *completeFilter) OnComplete(c Context) error {
	*f.done = true
	return nil
}

func TestFilterChain_Complete(t *testing.T) {
	t.Run("oncomplete runs for chained filters", func(t *testing.T) {
		t.Parallel()

		done := false
		mgr := newHttpFilterManager(fakeChainContext(t))
		mgr.AddFilter(&recordFilter{calls: &[]string{}, name: "x"})
		mgr.AddFilter(&completeFilter{done: &done})

		mgr.Complete()

		assert.True(t, done)
	})

	t.Run("composer chain completes", func(t *testing.T) {
		t.Parallel()

		done := false
		c := fakeChainContext(t)
		comp := &fakeComposer{filters: []Filter{&completeFilter{done: &done}}}

		mgr, err := buildFilterChainManager(c, func() Filter { return comp })
		require.NoError(t, err)

		mgr.Complete()

		assert.True(t, done)
	})
}

type misuseFunc func(c Context) error

func (f misuseFunc) OnRequestHeader(c Context) error  { return f(c) }
func (f misuseFunc) OnRequestBody(c Context) error    { return f(c) }
func (f misuseFunc) OnResponseHeader(c Context) error { return f(c) }
func (f misuseFunc) OnResponseBody(c Context) error   { return f(c) }

func TestFilterChain_MisusePerPhase(t *testing.T) {
	testcases := []struct {
		name   string
		access func(c Context) error
		encode bool
	}{
		{name: "response body in request phase", access: func(c Context) error {
			_ = c.ResponseBody()
			return nil
		}},
		{name: "request snapshot before load", access: func(c Context) error {
			_ = c.Request()
			return nil
		}},
		{name: "request body without data phase", access: func(c Context) error {
			_ = c.RequestBody()
			return nil
		}},
		{name: "request body in response phase", access: func(c Context) error {
			_ = c.RequestBody()
			return nil
		}, encode: true},
		{name: "response snapshot before load", access: func(c Context) error {
			_ = c.Response()
			return nil
		}, encode: true},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := fakeChainContext(t)
			mgr := newHttpFilterManager(c)
			mgr.SetErrorHandler(func(_ Context, err error) api.StatusType {
				return api.Continue
			})
			mgr.AddFilter(misuseFunc(tc.access))

			var res *HttpFilterResult
			require.NotPanics(t, func() {
				if tc.encode {
					res = mgr.ServeEncodeFilter(fakeEncodeHeadersPhase())
				} else {
					res = mgr.ServeDecodeFilter(fakeDecodeHeadersPhase())
				}
			})
			require.Error(t, res.Err)
			assert.ErrorIs(t, res.Err, ErrOperationNotPermitted)
		})
	}
}
