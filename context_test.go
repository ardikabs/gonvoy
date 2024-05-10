package gonvoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/require"
)

func fakeDummyContext(t *testing.T) Context {
	fc := mock_envoy.NewFilterCallbackHandler(t)
	c, err := newContext(fc)
	require.NoError(t, err)
	return c
}
