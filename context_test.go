package envoy

import (
	"bytes"
	"testing"

	"github.com/ardikabs/go-envoy/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_StoreAndLoad(t *testing.T) {
	fc := new(mocks.FilterCallbacks)
	ctx, err := NewContext(fc)
	require.NoError(t, err)

	source := bytes.NewReader([]byte("testing"))
	ctx.Store("foo", source)

	receiver := new(bytes.Reader)
	ok, err := ctx.Load("foo", &receiver)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, source, receiver)
}
