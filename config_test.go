package gonvoy

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
