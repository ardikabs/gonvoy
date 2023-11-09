package envoy

import (
	"testing"

	mock_envoy "github.com/ardikabs/go-envoy/test/mock/envoy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	h := &headerWriter{reqHeaderMap}
	assert.NotNil(t, h)
	assert.Equal(t, "bar", h.AsMap()["foo"])
	assert.Equal(t, "x-bar, x-foobar", h.AsMap()["x-foo"])
	assert.Equal(t, "x-foo-bar,x-foo-bar-2", h.AsMap()["x-foo-bar"])
}
