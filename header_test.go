package envoy

import (
	"testing"

	"github.com/ardikabs/go-envoy/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHaderAsMap(t *testing.T) {
	reqHeaderMap := new(mocks.RequestHeaderMap)
	reqHeaderMap.On("Range", mock.Anything).Return().Run(func(args mock.Arguments) {
		fn := args.Get(0).(func(key, value string) bool)
		headers := map[string][]string{
			"foo":       {"bar"},
			"x-foo":     {"x-bar", "x-foobar"},
			"x-foo-bar": {"x-foo-bar,x-foo-bar-2"},
		}
		for k, values := range headers {
			for _, v := range values {
				fn(k, v)
			}
		}
	})

	h := &header{reqHeaderMap}
	assert.NotNil(t, h)
	assert.Equal(t, "bar", h.AsMap()["foo"])
	assert.Equal(t, "x-bar, x-foobar", h.AsMap()["x-foo"])
	assert.Equal(t, "x-foo-bar,x-foo-bar-2", h.AsMap()["x-foo-bar"])
}
