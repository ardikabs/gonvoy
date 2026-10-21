package gonvoy

import (
	"testing"

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

	gc := newInternalConfig(ConfigOptions{
		MetricsPrefix: "PREFIX ",
	})

	gc.callbacks = cc
	gc.defineCounterMetric("foo_key=value_key1=value2")
	gc.defineGaugeMetric("foo_key=value_key1=value2")
}

type destroyRecorder struct {
	calls int
}

func (d *destroyRecorder) Destroy() { d.calls++ }

func TestInternalConfig_Destroy(t *testing.T) {
	t.Parallel()

	t.Run("calls user destroy", func(t *testing.T) {
		t.Parallel()

		rec := &destroyRecorder{}
		c := newInternalConfig(ConfigOptions{})
		c.filterConfig = rec

		require.NotPanics(t, func() { c.Destroy() })
		assert.Equal(t, 1, rec.calls)
	})

	t.Run("no destroy is a no-op", func(t *testing.T) {
		t.Parallel()

		c := newInternalConfig(ConfigOptions{})
		c.filterConfig = new(struct{ Name string })

		require.NotPanics(t, func() { c.Destroy() })
	})

	t.Run("nil config is a no-op", func(t *testing.T) {
		t.Parallel()

		c := newInternalConfig(ConfigOptions{})

		require.NotPanics(t, func() { c.Destroy() })
	})
}
