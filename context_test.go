package gonvoy

import (
	"bytes"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeDummyContext(t *testing.T) Context {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	cc := mock_envoy.NewConfigCallbackHandler(t)
	cfg := newGlobalConfig(cc, ConfigOptions{})
	c, err := newContext(fc, cfg)
	require.NoError(t, err)
	return c
}

func TestContext_StoreAndLoad(t *testing.T) {
	ctx := fakeDummyContext(t)

	source := bytes.NewReader([]byte("testing"))
	ctx.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := ctx.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
