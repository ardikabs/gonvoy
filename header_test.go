package gonvoy

import (
	"net/http"
	"testing"

	mock_envoy "github.com/ardikabs/gonvoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHeaderMapAsMap(t *testing.T) {
	reqHeaderMap := mock_envoy.NewRequestHeaderMap(t)
	reqHeaderMap.EXPECT().Range(mock.Anything).Return().Run(func(f func(string, string) bool) {
		headers := map[string][]string{
			"foo":       {"bar"},
			"x-foo":     {"x-bar", "x-foobar"},
			"x-foo-bar": {"x-foo-bar,x-foo-bar-2"},
		}
		for k, values := range headers {
			for _, v := range values {
				f(k, v)
			}
		}
	})

	h := &header{reqHeaderMap}
	assert.NotNil(t, h)
	assert.Equal(t, []string{"bar"}, h.AsMap()["foo"])
	assert.Equal(t, []string{"x-bar", "x-foobar"}, h.AsMap()["x-foo"])
	assert.Equal(t, []string{"x-foo-bar,x-foo-bar-2"}, h.AsMap()["x-foo-bar"])
}

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

	t.Run("new gateway headers with envoy header", func(t *testing.T) {
		reqHeaderMap := mock_envoy.NewRequestHeaderMap(t)
		reqHeaderMap.EXPECT().Range(mock.Anything).Return().Run(func(f func(string, string) bool) {
			headers := map[string][]string{
				"x-request-id": {"asdf12345"},
				"x-foo":        {"x-bar", "x-foobar"},
				"x-foo-bar":    {"x-foo-bar,x-foo-bar-2"},
			}
			for k, values := range headers {
				for _, v := range values {
					f(k, v)
				}
			}
		})
		reqHeaderMap.EXPECT().Method().Return(http.MethodGet)
		reqHeaderMap.EXPECT().Host().Return("foo.bar.com")
		reqHeaderMap.EXPECT().Path().Return("/foo/bar")

		fc := mock_envoy.NewFilterCallbackHandler(t)
		cfg := fakeConfig(t)

		ctx, err := newContext(fc, cfg)
		require.NoError(t, err)
		require.NotNil(t, ctx)

		ctx.SetRequestHeader(reqHeaderMap)

		headers := NewGatewayHeadersWithEnvoyHeader(ctx.RequestHeader())

		assert.Equal(t, "gateway", headers.Get("reporter"))
		assert.Equal(t, "asdf12345", headers.Get("x-request-id"))
	})
}
