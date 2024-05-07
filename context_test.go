package gonvoy

import (
	"bytes"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func fakeGlobalConfig(t *testing.T) Configuration {
	cc := mock_envoy.NewConfigCallbackHandler(t)
	return newGlobalConfig(cc, ConfigOptions{})
}

func fakeMetrics(t *testing.T) Metrics {
	cc := mock_envoy.NewConfigCallbackHandler(t)

	cm := mock_envoy.NewCounterMetric(t)
	cm.EXPECT().Increment(mock.Anything).Maybe()
	gm := mock_envoy.NewGaugeMetric(t)
	gm.EXPECT().Increment(mock.Anything).Maybe()

	cc.EXPECT().DefineGaugeMetric(mock.Anything).Return(cm).Maybe()
	cc.EXPECT().DefineCounterMetric(mock.Anything).Return(gm).Maybe()
	return newMetrics(cc.DefineCounterMetric, cc.DefineGaugeMetric, nil)
}

func TestContext_StoreAndLoad(t *testing.T) {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	ctx, err := newContext(fc)
	require.NoError(t, err)

	source := bytes.NewReader([]byte("testing"))
	ctx.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := ctx.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
