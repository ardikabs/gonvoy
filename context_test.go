package envoy_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ardikabs/go-envoy"
	"github.com/ardikabs/go-envoy/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_StoreAndLoad(t *testing.T) {
	fc := new(mocks.FilterCallbacks)
	ctx, err := envoy.NewContext(fc)
	require.NoError(t, err)

	s := bytes.NewReader([]byte("testing"))
	ctx.Store("foo", s)

	r := new(io.Reader)
	ok, err := ctx.Load("foo", &r)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, s, r)
}
