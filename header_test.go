package gonvoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGatewayHeaders(t *testing.T) {
	t.Run("ideal", func(t *testing.T) {
		headers := NewGatewayHeaders()

		assert.NotEmpty(t, headers)
		assert.Equal(t, "gateway", headers.Get("reporter"))
	})

	t.Run("additional headers, with a valid argument", func(t *testing.T) {
		headers := NewGatewayHeaders(
			"foo", "bar",
			"foobar", "barfoo",
			"loo", "bar",
		)

		assert.NotEmpty(t, headers)
		assert.Equal(t, "gateway", headers.Get("reporter"))
		assert.Equal(t, "bar", headers.Get("foo"))
		assert.Equal(t, "barfoo", headers.Get("foobar"))
		assert.Equal(t, "bar", headers.Get("loo"))
	})

	t.Run("expect additional headers, but argument is not valid because it contains odd arguments", func(t *testing.T) {
		headers := NewGatewayHeaders(
			"foo", "bar",
			"foobar",
		)

		assert.Len(t, headers, 1)
		assert.Equal(t, "", headers.Get("foo"))
		assert.Equal(t, "", headers.Get("foobar"))
	})
}
