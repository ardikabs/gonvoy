package gonvoy

import (
	"bytes"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// func fakeGlobalConfig(t *testing.T) Configuration {
// 	cc := mock_envoy.NewConfigCallbackHandler(t)
// 	return newGlobalConfig(cc, ConfigOptions{})
// }

func TestConfiguration_Metrics(t *testing.T) {
	cc := mock_envoy.NewConfigCallbackHandler(t)
	cc.EXPECT().DefineCounterMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "prefix_foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewCounterMetric(t))

	cc.EXPECT().DefineGaugeMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "prefix_foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewGaugeMetric(t))

	gc := newGlobalConfig(cc, ConfigOptions{
		MetricPrefix: "PREFIX ",
	})

	gc.metricCounter("foo_key=value_key1=value2")
	gc.metricGauge("foo_key=value_key1=value2")
}

func TestCache_StoreAndLoad(t *testing.T) {
	lc := NewCache()

	source := bytes.NewReader([]byte("testing"))
	lc.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := lc.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
