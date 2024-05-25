package gonvoy

import (
	"bytes"
	"testing"

	"github.com/ardikabs/gonvoy/pkg/errs"
	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfiguration_Metrics(t *testing.T) {
	cc := mock_envoy.NewConfigCallbackHandler(t)
	cc.EXPECT().DefineCounterMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "prefix_foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewCounterMetric(t))

	cc.EXPECT().DefineGaugeMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "prefix_foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewGaugeMetric(t))

	gc := newGlobalConfig(cc, ConfigOptions{
		MetricsPrefix: "PREFIX ",
	})

	gc.metricCounter("foo_key=value_key1=value2")
	gc.metricGauge("foo_key=value_key1=value2")
}

func TestCache_StoreAndLoad(t *testing.T) {
	lc := newCache()

	t.Run("pointer object", func(t *testing.T) {
		source := bytes.NewReader([]byte("testing"))
		lc.Store("foo", source)

		receiver := new(bytes.Reader)
		ok, err := lc.Load("foo", &receiver)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, source, receiver)
	})

	t.Run("literal object", func(t *testing.T) {
		type mystruct struct{}
		src := mystruct{}
		lc.Store("bar", src)

		dest := mystruct{}
		ok, err := lc.Load("bar", &dest)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, src, dest)
	})

	t.Run("a nil receiver, returns an error", func(t *testing.T) {
		ok, err := lc.Load("bar", nil)
		assert.False(t, ok)
		assert.ErrorIs(t, err, errs.ErrNilReceiver)
	})

	t.Run("receiver has incompatibility data type with the source, returns an error", func(t *testing.T) {
		type mystruct struct{}
		src := new(mystruct)
		lc.Store("foobar", src)

		dest := mystruct{}
		ok, err := lc.Load("foobar", &dest)
		assert.False(t, ok)
		assert.ErrorIs(t, err, errs.ErrIncompatibleReceiver)
	})

	t.Run("if no data found during a Load, then returns false without an error", func(t *testing.T) {
		dest := struct{}{}
		ok, err := lc.Load("data-not-exists", &dest)
		assert.False(t, ok)
		assert.NoError(t, err)
	})
}
