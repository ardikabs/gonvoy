package envoy

import (
	"bytes"
	"testing"

	mock_envoy "github.com/ardikabs/go-envoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_StoreAndLoad(t *testing.T) {
	mockCC := mock_envoy.NewConfigCallbackHandler(t)
	cfg := newConfig(nil, mockCC)

	source := bytes.NewReader([]byte("testing"))
	cfg.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := cfg.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
