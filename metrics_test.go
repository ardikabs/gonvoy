package gaetway

import (
	"testing"

	mock_envoy "github.com/ardikabs/gaetway/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// func fakeMetrics(t *testing.T) Metrics {
// 	cc := mock_envoy.NewConfigCallbackHandler(t)

// 	cm := mock_envoy.NewCounterMetric(t)
// 	cm.EXPECT().Increment(mock.Anything).Maybe()
// 	gm := mock_envoy.NewGaugeMetric(t)
// 	gm.EXPECT().Increment(mock.Anything).Maybe()

// 	cc.EXPECT().DefineGaugeMetric(mock.Anything).Return(cm).Maybe()
// 	cc.EXPECT().DefineCounterMetric(mock.Anything).Return(gm).Maybe()
// 	return newMetrics(cc.DefineCounterMetric, cc.DefineGaugeMetric, nil)
// }

func TestMetrics(t *testing.T) {
	cc := mock_envoy.NewConfigCallbackHandler(t)
	cc.EXPECT().DefineCounterMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewCounterMetric(t))

	cc.EXPECT().DefineGaugeMetric(mock.MatchedBy(func(name string) bool {
		return assert.Equal(t, "foo_key=value_key1=value2", name)
	})).Return(mock_envoy.NewGaugeMetric(t))

	m := newMetrics(cc.DefineCounterMetric, cc.DefineGaugeMetric, nil)
	m.Counter("foo", "key", "value", "key1", "value2")
	m.Gauge("foo", "key", "value", "key1", "value2")
}
