package gonvoy

import (
	"bytes"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeConfig(t *testing.T) Configuration {
	cc := mock_envoy.NewConfigCallbackHandler(t)
	return &config{callbacks: cc}
}

func TestContext_StoreAndLoad(t *testing.T) {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	cfg := fakeConfig(t)
	ctx, err := newContext(fc, cfg)
	require.NoError(t, err)

	source := bytes.NewReader([]byte("testing"))
	ctx.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := ctx.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
