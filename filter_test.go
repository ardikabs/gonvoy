package gonvoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeFilterConfig struct {
	Allow bool
}

func TestFilterConfig(t *testing.T) {
	t.Run("typed config", func(t *testing.T) {
		testcases := []struct {
			name        string
			config      *internalConfig
			expect      fakeFilterConfig
			expectError bool
		}{
			{
				name:        "matching pointer type",
				config:      &internalConfig{filterConfig: &fakeFilterConfig{Allow: true}},
				expect:      fakeFilterConfig{Allow: true},
				expectError: false,
			},
			{
				name:        "missing config",
				config:      &internalConfig{},
				expect:      fakeFilterConfig{},
				expectError: true,
			},
			{
				name:        "mismatched type",
				config:      &internalConfig{filterConfig: "not-a-struct"},
				expect:      fakeFilterConfig{},
				expectError: true,
			},
		}

		for _, tc := range testcases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c := fakeDummyContext(t, tc.config)
				got, err := FilterConfig[*fakeFilterConfig](c)

				if tc.expectError {
					require.Error(t, err)
					assert.Nil(t, got)
					return
				}

				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.expect, *got)
			})
		}
	})

	t.Run("missing config wraps not found", func(t *testing.T) {
		t.Parallel()

		c := fakeDummyContext(t, &internalConfig{})
		_, err := FilterConfig[*fakeFilterConfig](c)

		require.Error(t, err)
		assert.ErrorIs(t, err, errInternalConfigNotFound)
	})
}

func TestFilterCache(t *testing.T) {
	t.Parallel()

	c := fakeDummyContext(t, &internalConfig{internalCache: newInternalCache()})
	assert.True(t, c.cache == FilterCache(c))

	FilterCache(c).Store("key", "value")

	receiver := ""
	ok, err := FilterCache(c).Load("key", &receiver)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "value", receiver)
}
