package gaetway

import (
	"testing"

	mock_envoy "github.com/ardikabs/gaetway/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
